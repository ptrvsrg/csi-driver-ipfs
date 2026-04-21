// @ts-check

const config = {
    title: "csi-driver-ipfs",
    tagline: "CSI driver for IPFS-backed Kubernetes volumes",
    url: "https://ptrvsrg.github.io",
    baseUrl: "/csi-driver-ipfs/",
    organizationName: "ptrvsrg",
    projectName: "csi-driver-ipfs",
    onBrokenLinks: "throw",
    markdown: {
        hooks: {
            onBrokenMarkdownLinks: "warn",
        }
    },
    i18n: {
        defaultLocale: "en",
        locales: ["en"]
    },
    presets: [
        [
            "classic",
            {
                docs: {
                    path: "docs",
                    routeBasePath: "/",
                    sidebarPath: require.resolve("./sidebars.js")
                },
                blog: false,
                theme: {
                    customCss: require.resolve("./src/css/custom.css")
                }
            }
        ]
    ],
    themeConfig: {
        navbar: {
            title: "csi-driver-ipfs",
            logo: {
                alt: "IPFS logo",
                src: "img/ipfs-logo-1024-ice-text.png",
                height: 32
            },
            items: [
                {type: "docSidebar", sidebarId: "docsSidebar", label: "Docs", position: "left"},
                {href: "https://github.com/ptrvsrg/csi-driver-ipfs", label: "GitHub", position: "right"}
            ]
        },
        footer: {
            style: "dark",
            links: [
                {
                    title: "Project",
                    items: [
                        {label: "Repository", href: "https://github.com/ptrvsrg/csi-driver-ipfs"},
                        {label: "Charts index", href: "https://ptrvsrg.github.io/csi-driver-ipfs/charts/index.yaml"}
                    ]
                }
            ],
            copyright: `Copyright ${new Date().getFullYear()} ptrvsrg`
        }
    }
};

module.exports = config;
