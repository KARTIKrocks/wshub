import { useEffect, useRef, useState, useMemo } from 'react';
import { useVersion } from '../hooks/useVersion';

interface SidebarProps {
  open: boolean;
  onClose: () => void;
}

interface ChildItem {
  id: string;
  label: string;
  minVersion?: string;
}

interface SectionItem {
  id: string;
  label: string;
  minVersion?: string;
  children?: ChildItem[];
}

const sections: SectionItem[] = [
  { id: 'top', label: 'Overview' },
  {
    id: 'getting-started',
    label: 'Getting Started',
  },
  {
    id: 'hub',
    label: 'Hub',
    children: [
      { id: 'hub-creating', label: 'Creating a Hub' },
      { id: 'hub-options', label: 'Hub Options' },
      { id: 'hub-broadcasting', label: 'Broadcasting' },
      { id: 'hub-client-lookup', label: 'Client Lookup' },
      { id: 'hub-upgrade', label: 'Upgrade Options', minVersion: 'v1.1.0' },
      { id: 'hub-drop-policy', label: 'Drop Policy', minVersion: 'v1.1.0' },
      { id: 'hub-health', label: 'Health & Readiness', minVersion: 'v1.4.0' },
      { id: 'hub-drain', label: 'Graceful Drain', minVersion: 'v1.2.0' },
      { id: 'hub-shutdown', label: 'Graceful Shutdown' },
    ],
  },
  {
    id: 'client',
    label: 'Client',
    children: [
      { id: 'client-properties', label: 'Properties' },
      { id: 'client-sending', label: 'Sending Messages' },
      { id: 'client-metadata', label: 'Metadata' },
      { id: 'client-callbacks', label: 'Callbacks' },
      { id: 'client-closing', label: 'Closing' },
    ],
  },
  {
    id: 'messages',
    label: 'Messages',
    children: [
      { id: 'messages-type', label: 'Message Type' },
      { id: 'messages-handler', label: 'Message Handler' },
      { id: 'messages-raw-json', label: 'Pre-serialized JSON', minVersion: 'v1.1.3' },
    ],
  },
  {
    id: 'rooms',
    label: 'Rooms',
    children: [
      { id: 'rooms-joining', label: 'Joining & Leaving' },
      { id: 'rooms-broadcasting', label: 'Room Broadcasting' },
      { id: 'rooms-querying', label: 'Querying Rooms' },
    ],
  },
  {
    id: 'middleware',
    label: 'Middleware',
    children: [
      { id: 'middleware-chain', label: 'Middleware Chain' },
      { id: 'middleware-builtin', label: 'Built-in Middleware' },
      { id: 'middleware-custom', label: 'Custom Middleware' },
    ],
  },
  {
    id: 'router',
    label: 'Router',
    children: [
      { id: 'router-creating', label: 'Creating a Router' },
      { id: 'router-handlers', label: 'Registering Handlers' },
      { id: 'router-example', label: 'Full Example' },
    ],
  },
  {
    id: 'adapter',
    label: 'Adapters',
    minVersion: 'v1.1.0',
    children: [
      { id: 'adapter-interface', label: 'Adapter Interface' },
      { id: 'adapter-message', label: 'Adapter Message' },
      { id: 'adapter-redis', label: 'Redis Adapter' },
      { id: 'adapter-nats', label: 'NATS Adapter' },
      { id: 'adapter-how', label: 'How It Works' },
    ],
  },
  {
    id: 'presence',
    label: 'Presence',
    minVersion: 'v1.1.0',
    children: [
      { id: 'presence-enabling', label: 'Enabling Presence' },
      { id: 'presence-global', label: 'Global Counts' },
      { id: 'presence-example', label: 'Full Example' },
    ],
  },
  {
    id: 'hooks',
    label: 'Hooks',
    children: [
      { id: 'hooks-connection', label: 'Connection Hooks' },
      { id: 'hooks-message', label: 'Message Hooks' },
      { id: 'hooks-room', label: 'Room Hooks' },
    ],
  },
  {
    id: 'config',
    label: 'Configuration',
    children: [
      { id: 'config-defaults', label: 'Default Config' },
      { id: 'config-builder', label: 'Builder Methods' },
      { id: 'config-origins', label: 'Origin Checking' },
    ],
  },
  {
    id: 'limits',
    label: 'Limits',
    children: [
      { id: 'limits-connections', label: 'Connection Limits' },
      { id: 'limits-rooms', label: 'Room Limits' },
      { id: 'limits-rate', label: 'Rate Limiting' },
    ],
  },
  {
    id: 'metrics',
    label: 'Metrics',
    children: [
      { id: 'metrics-interface', label: 'Metrics Interface' },
      { id: 'metrics-debug', label: 'Debug Metrics' },
    ],
  },
  {
    id: 'errors',
    label: 'Errors',
    children: [
      { id: 'errors-connection', label: 'Connection Errors' },
      { id: 'errors-hub-state', label: 'Hub State Errors', minVersion: 'v1.2.0' },
      { id: 'errors-client', label: 'Client Errors' },
      { id: 'errors-room', label: 'Room Errors' },
      { id: 'errors-limits', label: 'Limit Errors' },
    ],
  },
];

function updateHash(id: string) {
  const url = new URL(window.location.href);
  if (id === 'top') {
    url.hash = '';
  } else {
    url.hash = id;
  }
  if (window.location.hash !== url.hash) {
    history.replaceState(null, '', url.toString());
  }
}

export default function Sidebar({ open, onClose }: SidebarProps) {
  const { minVersion } = useVersion();

  // Filter sections and their children by minVersion
  const filteredSections = useMemo(() => {
    return sections
      .filter((s) => !s.minVersion || minVersion(s.minVersion))
      .map((s) => {
        if (!s.children) return s;
        const filteredChildren = s.children.filter(
          (c) => !c.minVersion || minVersion(c.minVersion),
        );
        return { ...s, children: filteredChildren.length > 0 ? filteredChildren : undefined };
      });
  }, [minVersion]);

  // All navigable ids: section ids + sub-topic ids
  const allIds = useMemo(
    () =>
      filteredSections.flatMap((s) =>
        s.children ? [s.id, ...s.children.map((c) => c.id)] : [s.id],
      ),
    [filteredSections],
  );

  // Map sub-topic id → parent section id
  const parentMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const s of filteredSections) {
      if (s.children) {
        for (const c of s.children) {
          map.set(c.id, s.id);
        }
      }
    }
    return map;
  }, [filteredSections]);

  const [active, setActive] = useState(() => {
    const hash = window.location.hash.slice(1);
    return hash && document.getElementById(hash) ? hash : 'top';
  });
  const [expanded, setExpanded] = useState<string | null>(() => {
    const hash = window.location.hash.slice(1);
    const parent = parentMap.get(hash);
    if (parent) return parent;
    const section = filteredSections.find((s) => s.id === hash);
    return section?.children ? hash : null;
  });

  const visibleSet = useRef(new Set<string>());
  const isScrollingTo = useRef<string | null>(null);
  const scrollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Scroll to hash on initial load
  useEffect(() => {
    const hash = window.location.hash.slice(1);
    if (hash) {
      setTimeout(() => {
        document.getElementById(hash)?.scrollIntoView({ behavior: 'smooth' });
      }, 100);
    }
  }, []);

  const setActiveAndHash = (id: string) => {
    setActive(id);
    updateHash(id);

    // Auto-expand the relevant section (only one at a time)
    const parent = parentMap.get(id);
    if (parent) {
      setExpanded(parent);
    } else {
      const section = filteredSections.find((s) => s.id === id);
      if (section?.children) {
        setExpanded(id);
      }
    }
  };

  useEffect(() => {
    const updateActive = () => {
      if (isScrollingTo.current) return;
      if (visibleSet.current.size === 0) return;

      // Find the visible element whose top is closest to (and >= ) the viewport top
      // This ensures sub-items win over their large parent sections
      const navbarOffset = 80;
      let bestId: string | null = null;
      let bestDistance = Infinity;

      for (const id of visibleSet.current) {
        const el = document.getElementById(id);
        if (!el) continue;
        const rect = el.getBoundingClientRect();
        const distance = Math.abs(rect.top - navbarOffset);
        if (distance < bestDistance) {
          bestDistance = distance;
          bestId = id;
        }
      }

      if (bestId) {
        setActiveAndHash(bestId);
      }
    };

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            visibleSet.current.add(entry.target.id);
          } else {
            visibleSet.current.delete(entry.target.id);
          }
        }
        updateActive();
      },
      { rootMargin: '-80px 0px -40% 0px', threshold: 0.1 }
    );

    allIds.forEach((id) => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });

    const onScroll = () => {
      if (!isScrollingTo.current) return;
      if (scrollTimer.current) clearTimeout(scrollTimer.current);
      scrollTimer.current = setTimeout(() => {
        isScrollingTo.current = null;
        updateActive();
      }, 150);
    };

    window.addEventListener('scroll', onScroll, { passive: true });

    return () => {
      observer.disconnect();
      window.removeEventListener('scroll', onScroll);
      if (scrollTimer.current) clearTimeout(scrollTimer.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allIds]);

  const handleClick = (id: string) => {
    isScrollingTo.current = id;
    setActiveAndHash(id);
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
    onClose();
  };

  const toggleExpand = (id: string) => {
    setExpanded((prev) => (prev === id ? null : id));
  };

  const isActive = (id: string) => active === id;

  const isSectionActive = (section: SectionItem) =>
    active === section.id ||
    (section.children?.some((c) => c.id === active) ?? false);

  const sectionClass = (section: SectionItem) =>
    `flex items-center justify-between w-full px-3 py-1.5 rounded-md text-sm transition-colors cursor-pointer ${
      isSectionActive(section)
        ? 'text-primary font-medium'
        : 'text-text-muted hover:text-text hover:bg-bg-card'
    }`;

  const subItemClass = (id: string) =>
    `block w-full text-left pl-6 pr-3 py-1 rounded-md text-xs transition-colors cursor-pointer ${
      isActive(id)
        ? 'bg-primary/10 text-primary font-medium'
        : 'text-text-muted hover:text-text hover:bg-bg-card'
    }`;

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 bg-black/50 z-30 md:hidden"
          onClick={onClose}
        />
      )}

      <aside
        className={`fixed top-16 left-0 bottom-0 w-64 bg-bg-sidebar border-r border-border overflow-y-auto z-40 transition-transform ${
          open ? 'translate-x-0' : '-translate-x-full'
        } md:translate-x-0`}
      >
        <nav className="p-4 space-y-0.5">
          {filteredSections.map((section) =>
            section.children ? (
              <div key={section.id}>
                <button
                  onClick={() => {
                    if (expanded === section.id) {
                      toggleExpand(section.id);
                    } else {
                      handleClick(section.id);
                    }
                  }}
                  className={sectionClass(section)}
                >
                  <span>{section.label}</span>
                  <svg
                    className={`w-3.5 h-3.5 transition-transform ${
                      expanded === section.id ? 'rotate-90' : ''
                    }`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5l7 7-7 7"
                    />
                  </svg>
                </button>
                {expanded === section.id && (
                  <div className="mt-0.5 space-y-0.5">
                    {section.children.map((child) => (
                      <button
                        key={child.id}
                        onClick={() => handleClick(child.id)}
                        className={subItemClass(child.id)}
                      >
                        {child.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <button
                key={section.id}
                onClick={() => handleClick(section.id)}
                className={`block w-full text-left px-3 py-1.5 rounded-md text-sm transition-colors cursor-pointer ${
                  isActive(section.id)
                    ? 'bg-primary/10 text-primary font-medium'
                    : 'text-text-muted hover:text-text hover:bg-bg-card'
                }`}
              >
                {section.label}
              </button>
            )
          )}
        </nav>
      </aside>
    </>
  );
}
