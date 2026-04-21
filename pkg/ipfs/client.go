// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ipfs implements a kubo (go-ipfs) HTTP RPC client for pins, Unixfs, MFS, and repository stats.
package ipfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/boxo/files"
	boxopath "github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	kuborpc "github.com/ipfs/kubo/client/rpc"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

var (
	ErrEmptyAddress = errors.New("IPFS API address is empty")
)

// RepoStatResult is the decoded JSON payload from the IPFS repo/stat API.
type RepoStatResult struct {
	RepoSize   uint64 `json:"RepoSize"`
	StorageMax uint64 `json:"StorageMax"`
	NumObjects uint64 `json:"NumObjects"`
}

// MFSEntry describes one child in an MFS directory listing.
type MFSEntry struct {
	Name string
	Type int
	Hash string
	Size uint64
}

// Client is the IPFS kubo RPC surface required by the CSI driver.
type Client interface {
	// Ping checks that the API is reachable by fetching the local node identity.
	// ctx cancels the RPC. It returns nil on success or a non-nil error on transport/API failure.
	Ping(ctx context.Context) error

	// StatCID checks whether cidStr resolves to a Unixfs node via the API.
	// ctx cancels the RPC. It returns (true, nil) when content exists, (false, nil) when Get fails
	// (not found / unreachable), or (false, err) when cidStr cannot be parsed.
	StatCID(ctx context.Context, cidStr string) (bool, error)

	// GetCIDContent materializes the DAG at cidStr under destPath (directory must exist or be creatable as files).
	// ctx cancels the download. It returns nil on success or a non-nil error on parse, fetch, or write failure.
	GetCIDContent(ctx context.Context, cidStr string, destPath string) error

	// AddPath adds localPath (file or directory) to Unixfs and returns the resulting root CID string.
	// ctx cancels the add. It returns the CID or a non-nil error.
	AddPath(ctx context.Context, localPath string) (string, error)

	// PinCID adds a pin for cidStr in the local repo.
	// ctx cancels the RPC. It returns nil on success or a non-nil error.
	PinCID(ctx context.Context, cidStr string) error

	// UnpinCID removes a pin for cidStr in the local repo.
	// ctx cancels the RPC. It returns nil on success or a non-nil error.
	UnpinCID(ctx context.Context, cidStr string) error

	// MkdirMFS creates mfsPath under MFS, creating parents when needed.
	// ctx cancels the RPC. It returns nil on success or a non-nil error.
	MkdirMFS(ctx context.Context, mfsPath string) error

	// RmMFS removes mfsPath from MFS recursively.
	// ctx cancels the RPC. It returns nil on success or a non-nil error.
	RmMFS(ctx context.Context, mfsPath string) error

	// ExportMFS resolves mfsPath to its root hash and downloads it into localPath.
	// ctx cancels work. It returns nil on success or a non-nil error.
	ExportMFS(ctx context.Context, mfsPath string, localPath string) error

	// ImportToMFS replaces mfsPath with a tree mirrored from localPath (walk + per-file add + MFS cp).
	// ctx cancels work. It returns nil on success or a non-nil error.
	ImportToMFS(ctx context.Context, localPath string, mfsPath string) error

	// ResolveMFSCID returns the hash string for mfsPath from files/stat (typically a CID).
	// ctx cancels the RPC. It returns the hash or a non-nil error.
	ResolveMFSCID(ctx context.Context, mfsPath string) (string, error)

	// GetRepoStat fetches repository size and limits from repo/stat.
	// ctx cancels the RPC. It returns RepoStatResult or a non-nil error.
	GetRepoStat(ctx context.Context) (*RepoStatResult, error)

	// ReadCID streams a single Unixfs file node for cidStr. The caller must close the ReadCloser.
	// If the CID denotes a directory, ReadCID returns an error.
	// ctx cancels the underlying fetch. It returns a ReadCloser or a non-nil error.
	ReadCID(ctx context.Context, cidStr string) (io.ReadCloser, error)
}

type client struct {
	logger  zerolog.Logger
	httpAPI *kuborpc.HttpApi
}

// NewClient dials the kubo HTTP API at apiAddr (multiaddr form, e.g. "/ip4/127.0.0.1/tcp/5001").
//
// logger receives operational debug/info lines for this client.
// apiAddr must be non-empty or ErrEmptyAddress is returned.
// On success it returns a Client implementation; otherwise it returns nil and a wrapped error.
func NewClient(logger zerolog.Logger, apiAddr string) (Client, error) {
	if apiAddr == "" {
		return nil, ErrEmptyAddress
	}

	ma, err := multiaddr.NewMultiaddr(apiAddr)
	if err != nil {
		return nil, fmt.Errorf("parse API multiaddr %q: %w", apiAddr, err)
	}

	httpAPI, err := kuborpc.NewApi(ma)
	if err != nil {
		return nil, fmt.Errorf("create kubo RPC client for %s: %w", apiAddr, err)
	}

	logger.Info().Str("addr", apiAddr).Msg("IPFS client created")

	return &client{
		logger:  logger,
		httpAPI: httpAPI,
	}, nil
}

func (c *client) Ping(ctx context.Context) error {
	c.logger.Debug().Msg("ping")

	key, err := c.httpAPI.Key().Self(ctx)
	if err != nil {
		return fmt.Errorf("check IPFS daemon health: %w", err)
	}

	c.logger.Debug().
		Str("peer_id", key.ID().String()).
		Msg("ping successful")

	return nil
}

func (c *client) StatCID(ctx context.Context, cidStr string) (bool, error) {
	c.logger.Debug().Str("cid", cidStr).Msg("stat CID")

	p, err := parseCIDPath(cidStr)
	if err != nil {
		return false, fmt.Errorf("parse CID path for stat: %w", err)
	}

	_, err = c.httpAPI.Unixfs().Get(ctx, p)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.logger.Debug().
				Str("cid", cidStr).
				Err(err).
				Msg("CID not found")
			return false, nil
		}
		c.logger.Debug().
			Str("cid", cidStr).
			Err(err).
			Msg("stat CID failed")
		return false, fmt.Errorf("stat CID %s: %w", cidStr, err)
	}

	c.logger.Debug().Str("cid", cidStr).Msg("CID exists")
	return true, nil
}

func (c *client) GetCIDContent(ctx context.Context, cidStr string, destPath string) error {
	c.logger.Debug().
		Str("cid", cidStr).
		Str("dest", destPath).
		Msg("download CID content")

	p, err := parseCIDPath(cidStr)
	if err != nil {
		return fmt.Errorf("parse CID path for download: %w", err)
	}

	node, err := c.httpAPI.Unixfs().Get(ctx, p)
	if err != nil {
		return fmt.Errorf("fetch CID %s from IPFS: %w", cidStr, err)
	}

	if err := writeNodeToDir(node, destPath); err != nil {
		return fmt.Errorf("write CID %s content to %s: %w", cidStr, destPath, err)
	}

	c.logger.Debug().
		Str("cid", cidStr).
		Str("dest", destPath).
		Msg("CID content downloaded")
	return nil
}

// writeNodeToDir writes IPFS node content into an existing directory.
// If node is a directory, its children are written inside destPath.
// If node is a file, it is written as destPath/filename.
func writeNodeToDir(nd files.Node, destPath string) error {
	switch n := nd.(type) {
	case files.Directory:
		iter := n.Entries()
		for iter.Next() {
			childName := iter.Name()
			childPath := filepath.Join(destPath, childName)
			childNode := iter.Node()

			if err := files.WriteTo(childNode, childPath); err != nil {
				return fmt.Errorf("write child %s to %s: %w", childName, childPath, err)
			}
		}
		if err := iter.Err(); err != nil {
			return fmt.Errorf("iterate directory entries: %w", err)
		}
		return nil

	case files.File:
		// Single file — write it inside destPath as "data"
		outPath := filepath.Join(destPath, "data")
		return files.WriteTo(n, outPath)

	default:
		return fmt.Errorf("unsupported IPFS node type %T", nd)
	}
}

func (c *client) AddPath(ctx context.Context, localPath string) (string, error) {
	c.logger.Debug().Str("path", localPath).Msg("add to IPFS")

	stat, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("stat %s before IPFS add: %w", localPath, err)
	}

	var node files.Node

	if stat.IsDir() {
		node, err = files.NewSerialFile(localPath, false, stat)
		if err != nil {
			return "", fmt.Errorf("create file node for directory %s: %w", localPath, err)
		}
	} else {
		rootDir := filepath.Dir(localPath)
		root, err := os.OpenRoot(rootDir)
		if err != nil {
			return "", fmt.Errorf("open root %s for IPFS add: %w", rootDir, err)
		}

		f, err := root.Open(filepath.Base(localPath))
		if err != nil {
			if err := root.Close(); err != nil {
				return "", fmt.Errorf("close root %s for IPFS add: %w", rootDir, err)
			}

			return "", fmt.Errorf("open file %s for IPFS add: %w", localPath, err)
		}
		// files.NewReaderFile takes ownership of the file handle; the bounded root
		// can be released once the file descriptor is opened.
		if rootCloseErr := root.Close(); rootCloseErr != nil {
			if fileCloseErr := f.Close(); fileCloseErr != nil {
				return "", fmt.Errorf("close file %s for IPFS add after root close failure: %w", localPath, fileCloseErr)
			}
			return "", fmt.Errorf("close root %s after IPFS add open: %w", rootDir, rootCloseErr)
		}

		node = files.NewReaderFile(f)
	}

	resolved, err := c.httpAPI.Unixfs().Add(ctx, node, options.Unixfs.Pin(false, ""))
	if err != nil {
		return "", fmt.Errorf("add %s to IPFS: %w", localPath, err)
	}

	cidStr := resolved.RootCid().String()

	c.logger.Debug().
		Str("path", localPath).
		Str("cid", cidStr).
		Bool("is_dir", stat.IsDir()).
		Msg("added to IPFS")

	return cidStr, nil
}

func (c *client) PinCID(ctx context.Context, cidStr string) error {
	c.logger.Debug().Str("cid", cidStr).Msg("pin CID")

	p, err := parseCIDPath(cidStr)
	if err != nil {
		return fmt.Errorf("parse CID path for pin: %w", err)
	}

	if err := c.httpAPI.Pin().Add(ctx, p); err != nil {
		return fmt.Errorf("pin CID %s: %w", cidStr, err)
	}

	c.logger.Debug().Str("cid", cidStr).Msg("CID pinned")
	return nil
}

func (c *client) UnpinCID(ctx context.Context, cidStr string) error {
	c.logger.Debug().Str("cid", cidStr).Msg("unpin CID")

	p, err := parseCIDPath(cidStr)
	if err != nil {
		return fmt.Errorf("parse CID path for unpin: %w", err)
	}

	if err := c.httpAPI.Pin().Rm(ctx, p); err != nil {
		return fmt.Errorf("unpin CID %s: %w", cidStr, err)
	}

	c.logger.Debug().Str("cid", cidStr).Msg("CID unpinned")
	return nil
}

func (c *client) MkdirMFS(ctx context.Context, mfsPath string) error {
	c.logger.Debug().Str("path", mfsPath).Msg("create MFS directory")

	resp, err := c.httpAPI.Request("files/mkdir").
		Arguments(mfsPath).
		Option("parents", true).
		Send(ctx)
	if err != nil {
		return fmt.Errorf("send MFS mkdir for %s: %w", mfsPath, err)
	}
	defer func(resp *kuborpc.Response) {
		if err := resp.Close(); err != nil {
			c.logger.Warn().Err(err).Msg("close response body")
		}
	}(resp)

	if resp.Error != nil {
		return fmt.Errorf("execute MFS mkdir %s: %w", mfsPath, resp.Error)
	}

	c.logger.Debug().Str("path", mfsPath).Msg("MFS directory created")
	return nil
}

func (c *client) RmMFS(ctx context.Context, mfsPath string) error {
	c.logger.Debug().Str("path", mfsPath).Msg("remove MFS path")

	resp, err := c.httpAPI.Request("files/rm").
		Arguments(mfsPath).
		Option("recursive", true).
		Send(ctx)
	if err != nil {
		return fmt.Errorf("send MFS rm for %s: %w", mfsPath, err)
	}
	defer func(resp *kuborpc.Response) {
		if err := resp.Close(); err != nil {
			c.logger.Warn().Err(err).Msg("close response body")
		}
	}(resp)

	if resp.Error != nil {
		return fmt.Errorf("execute MFS rm %s: %w", mfsPath, resp.Error)
	}

	c.logger.Debug().Str("path", mfsPath).Msg("MFS path removed")
	return nil
}

// mfsStatResult holds the parsed response from files/stat.
type mfsStatResult struct {
	Hash string `json:"Hash"`
	Size uint64 `json:"Size"`
	Type string `json:"Type"`
}

// statMFS performs files/stat on the given MFS path.
func (c *client) statMFS(ctx context.Context, mfsPath string) (*mfsStatResult, error) {
	resp, err := c.httpAPI.Request("files/stat").
		Arguments(mfsPath).
		Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("send MFS stat for %s: %w", mfsPath, err)
	}
	defer func(resp *kuborpc.Response) {
		if err := resp.Close(); err != nil {
			c.logger.Warn().Err(err).Msg("close response body")
		}
	}(resp)

	if resp.Error != nil {
		return nil, fmt.Errorf("execute MFS stat %s: %w", mfsPath, resp.Error)
	}

	result := new(mfsStatResult)

	dec := json.NewDecoder(resp.Output)
	if err := dec.Decode(result); err != nil {
		return nil, fmt.Errorf("decode MFS stat response for %s: %w", mfsPath, err)
	}

	return result, nil
}

// cpMFS copies content within IPFS/MFS using files/cp.
func (c *client) cpMFS(ctx context.Context, src string, dst string) error {
	resp, err := c.httpAPI.Request("files/cp").
		Arguments(src, dst).
		Send(ctx)
	if err != nil {
		return fmt.Errorf("send MFS cp %s -> %s: %w", src, dst, err)
	}
	defer func(resp *kuborpc.Response) {
		if err := resp.Close(); err != nil {
			c.logger.Warn().Err(err).Msg("close response body")
		}
	}(resp)

	if resp.Error != nil {
		return fmt.Errorf("execute MFS cp %s -> %s: %w", src, dst, resp.Error)
	}

	return nil
}

func (c *client) ExportMFS(ctx context.Context, mfsPath string, localPath string) error {
	c.logger.Debug().
		Str("mfs_path", mfsPath).
		Str("local_path", localPath).
		Msg("export MFS content")

	stat, err := c.statMFS(ctx, mfsPath)
	if err != nil {
		return fmt.Errorf("resolve MFS path %s for export: %w", mfsPath, err)
	}

	c.logger.Debug().
		Str("mfs_path", mfsPath).
		Str("cid", stat.Hash).
		Uint64("size", stat.Size).
		Msg("MFS path resolved")

	if err := c.GetCIDContent(ctx, stat.Hash, localPath); err != nil {
		return fmt.Errorf(
			"download MFS content %s (CID %s) to %s: %w",
			mfsPath, stat.Hash, localPath, err,
		)
	}

	c.logger.Debug().
		Str("mfs_path", mfsPath).
		Str("local_path", localPath).
		Msg("MFS content exported")

	return nil
}

func (c *client) ImportToMFS(ctx context.Context, localPath string, mfsPath string) error {
	c.logger.Debug().
		Str("local_path", localPath).
		Str("mfs_path", mfsPath).
		Msg("import to MFS")

	// Clean the target MFS directory; ignore errors because it may not exist yet
	if err := c.RmMFS(ctx, mfsPath); err != nil {
		c.logger.Warn().Err(err).Str("mfs_path", mfsPath).Msg("remove MFS directory for import")
	}

	if err := c.MkdirMFS(ctx, mfsPath); err != nil {
		return fmt.Errorf("recreate MFS directory %s for import: %w", mfsPath, err)
	}

	var fileCount int
	root, err := os.OpenRoot(localPath)
	if err != nil {
		return fmt.Errorf("open root %s for MFS import: %w", localPath, err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			c.logger.Warn().Err(err).Str("local_path", localPath).Msg("close import root")
		}
	}()

	err = filepath.Walk(
		localPath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("walk %s: %w", path, walkErr)
			}

			relPath, err := filepath.Rel(localPath, path)
			if err != nil {
				return fmt.Errorf("compute relative path %s from %s: %w", path, localPath, err)
			}

			if relPath == "." {
				return nil
			}

			mfsTarget := filepath.Join(mfsPath, relPath)

			if info.IsDir() {
				c.logger.Debug().Str("mfs_path", mfsTarget).Msg("create MFS subdirectory")
				if err := c.MkdirMFS(ctx, mfsTarget); err != nil {
					return fmt.Errorf("create MFS subdirectory %s: %w", mfsTarget, err)
				}
				return nil
			}

			c.logger.Debug().
				Str("file", path).
				Int64("size", info.Size()).
				Msg("import file")

			f, err := root.Open(relPath)
			if err != nil {
				return fmt.Errorf("open %s for import: %w", path, err)
			}
			defer func(f *os.File) {
				if err := f.Close(); err != nil {
					c.logger.Warn().Err(err).Msg("close file body")
				}
			}(f)

			node := files.NewReaderFile(f)
			resolved, err := c.httpAPI.Unixfs().Add(ctx, node, options.Unixfs.Pin(false, ""))
			if err != nil {
				return fmt.Errorf("add %s to IPFS during import: %w", path, err)
			}

			addedCID := resolved.RootCid().String()

			c.logger.Debug().
				Str("file", path).
				Str("cid", addedCID).
				Str("mfs_target", mfsTarget).
				Msg("copy to MFS")

			if err := c.cpMFS(ctx, "/ipfs/"+addedCID, mfsTarget); err != nil {
				return fmt.Errorf("copy CID %s to MFS %s: %w", addedCID, mfsTarget, err)
			}

			fileCount++
			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("import %s to MFS %s: %w", localPath, mfsPath, err)
	}

	c.logger.Debug().
		Str("local_path", localPath).
		Str("mfs_path", mfsPath).
		Int("file_count", fileCount).
		Msg("MFS import completed")

	return nil
}

func (c *client) ResolveMFSCID(ctx context.Context, mfsPath string) (string, error) {
	c.logger.Debug().Str("mfs_path", mfsPath).Msg("resolve MFS CID")

	stat, err := c.statMFS(ctx, mfsPath)
	if err != nil {
		return "", fmt.Errorf("resolve CID for MFS path %s: %w", mfsPath, err)
	}

	c.logger.Debug().
		Str("mfs_path", mfsPath).
		Str("cid", stat.Hash).
		Msg("MFS CID resolved")

	return stat.Hash, nil
}

func (c *client) GetRepoStat(ctx context.Context) (*RepoStatResult, error) {
	c.logger.Debug().Msg("fetch repo statistics")

	resp, err := c.httpAPI.Request("repo/stat").Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("request IPFS repo stat: %w", err)
	}
	defer func(resp *kuborpc.Response) {
		if err := resp.Close(); err != nil {
			c.logger.Warn().Err(err).Msg("close response body")
		}
	}(resp)

	if resp.Error != nil {
		return nil, fmt.Errorf("execute IPFS repo stat: %w", resp.Error)
	}

	stat := new(RepoStatResult)

	dec := json.NewDecoder(resp.Output)
	if err := dec.Decode(stat); err != nil {
		return nil, fmt.Errorf("decode repo stat response: %w", err)
	}

	c.logger.Debug().
		Uint64("repo_size", stat.RepoSize).
		Uint64("storage_max", stat.StorageMax).
		Uint64("num_objects", stat.NumObjects).
		Msg("repo statistics fetched")

	return stat, nil
}

func (c *client) ReadCID(ctx context.Context, cidStr string) (io.ReadCloser, error) {
	c.logger.Debug().Str("cid", cidStr).Msg("open CID stream")

	p, err := parseCIDPath(cidStr)
	if err != nil {
		return nil, fmt.Errorf("parse CID path for read: %w", err)
	}

	node, err := c.httpAPI.Unixfs().Get(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("fetch CID %s for read: %w", cidStr, err)
	}

	f, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("CID %s is a directory, not a readable file", cidStr)
	}

	c.logger.Debug().Str("cid", cidStr).Msg("CID stream opened")
	return f, nil
}

// parseCIDPath converts a CID string into a boxo immutable path (/ipfs/<cid>).
func parseCIDPath(cidStr string) (boxopath.Path, error) {
	c, err := cid.Decode(cidStr)
	if err != nil {
		return nil, fmt.Errorf("decode CID %q: %w", cidStr, err)
	}
	return boxopath.FromCid(c), nil
}
