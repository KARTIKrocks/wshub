import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Mirrors the module layout of the package itself: core types first
 * (hub → client → messages → rooms), then the composition layer
 * (middleware, router, hooks), then scaling, then operational concerns.
 */
const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Core',
      collapsed: false,
      items: ['hub', 'client', 'messages', 'rooms'],
    },
    {
      type: 'category',
      label: 'Composition',
      collapsed: false,
      items: ['middleware', 'router', 'hooks'],
    },
    {
      type: 'category',
      label: 'Scaling',
      collapsed: false,
      items: ['adapters', 'presence'],
    },
    {
      type: 'category',
      label: 'Operations',
      collapsed: false,
      items: ['configuration', 'limits', 'metrics', 'errors'],
    },
  ],
};

export default sidebars;
