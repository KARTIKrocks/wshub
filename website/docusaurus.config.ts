import fs from 'node:fs';
import path from 'node:path';
import type * as Preset from '@docusaurus/preset-classic';
import type { Config } from '@docusaurus/types';
import { themes as prismThemes } from 'prism-react-renderer';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Versioning policy — see VERSIONING.md for the full runbook.
 *
 * Snapshots are cut per MINOR release ("1.7"), never per patch. A patch that
 * changes documented behaviour is edited into the existing snapshot in place.
 * Cutting per patch would multiply the docs tree by every release and is the
 * main reason versioned sites become unmaintainable.
 *
 * Only the newest MAX_LIVE_VERSIONS snapshots are built. Older ones stay in
 * git (readable at their tag) but are dropped from the site so build time and
 * search index size stay flat as releases accumulate.
 */
const MAX_LIVE_VERSIONS = 4;

const versionsFile = path.resolve(__dirname, 'versions.json');
const allVersions: string[] = fs.existsSync(versionsFile)
  ? JSON.parse(fs.readFileSync(versionsFile, 'utf8'))
  : [];

/**
 * `DOCS_FAST_BUILD=true` builds only the in-progress docs. Used by `start` and
 * by the PR build check, where rebuilding every historical version is wasted
 * work. Production deploys build the full live set.
 */
const fastBuild = process.env.DOCS_FAST_BUILD === 'true';
const liveVersions = allVersions.slice(0, MAX_LIVE_VERSIONS);
const includedVersions = fastBuild
  ? ['current', ...allVersions.slice(0, 1)]
  : ['current', ...liveVersions];

const config: Config = {
  title: 'wshub',
  tagline: 'Production-ready Go WebSocket hub',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
    // Rspack/SWC build pipeline — matters here because build time scales with
    // the number of versioned doc trees.
    faster: true,
  },

  url: 'https://kartikrocks.github.io',
  baseUrl: '/wshub/',

  organizationName: 'KARTIKrocks',
  projectName: 'wshub',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/KARTIKrocks/wshub/tree/main/website/',
          // `current` is the working copy on main — it documents unreleased
          // changes and is served at /docs/next/. The newest snapshot in
          // versions.json is what /docs/ serves, so the default reader always
          // lands on released behaviour.
          versions: {
            current: {
              label: 'Next (unreleased)',
              path: 'next',
              banner: 'unreleased',
            },
          },
          onlyIncludeVersions: includedVersions,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
        sitemap: {
          lastmod: 'date',
          changefreq: 'weekly',
          priority: 0.5,
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/wshub-social-card.png',
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'wshub',
      logo: {
        alt: 'wshub',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownItemsAfter: [
            {
              href: 'https://github.com/KARTIKrocks/wshub/releases',
              label: 'All releases',
            },
          ],
        },
        {
          href: 'https://pkg.go.dev/github.com/KARTIKrocks/wshub',
          label: 'API Reference',
          position: 'right',
        },
        {
          href: 'https://github.com/KARTIKrocks/wshub',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Docs',
          items: [
            { label: 'Getting Started', to: '/docs/getting-started' },
            { label: 'Hub', to: '/docs/hub' },
            { label: 'Multi-Node Adapters', to: '/docs/adapters' },
          ],
        },
        {
          title: 'Reference',
          items: [
            {
              label: 'pkg.go.dev',
              href: 'https://pkg.go.dev/github.com/KARTIKrocks/wshub',
            },
            {
              label: 'Changelog',
              href: 'https://github.com/KARTIKrocks/wshub/blob/main/CHANGELOG.md',
            },
            {
              label: 'Releases',
              href: 'https://github.com/KARTIKrocks/wshub/releases',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/KARTIKrocks/wshub',
            },
            {
              label: 'Issues',
              href: 'https://github.com/KARTIKrocks/wshub/issues',
            },
            {
              label: 'Contributing',
              href: 'https://github.com/KARTIKrocks/wshub/blob/main/CONTRIBUTING.md',
            },
          ],
        },
      ],
      copyright: `wshub is open source under the MIT License. Copyright © ${new Date().getFullYear()}.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['go', 'bash', 'json', 'yaml'],
    },
    // Algolia DocSearch — apply at https://docsearch.algolia.com/apply/
    // Uncomment and fill in the credentials once the application is approved.
    // algolia: {
    //   appId: 'YOUR_APP_ID',
    //   apiKey: 'YOUR_SEARCH_API_KEY',
    //   indexName: 'wshub',
    //   contextualSearch: true,
    // },
  } satisfies Preset.ThemeConfig,
};

export default config;
