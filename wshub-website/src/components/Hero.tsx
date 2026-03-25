import { useState, useMemo } from 'react';
import { useVersion } from '../hooks/useVersion';

interface Feature {
  title: string;
  desc: string;
  minVersion?: string;
}

const features: Feature[] = [
  { title: 'Production Ready', desc: 'Proper concurrency, graceful shutdown, error handling' },
  { title: 'Multi-Node Support', desc: 'Scale horizontally with Redis or NATS adapters and presence gossip', minVersion: 'v1.1.0' },
  { title: 'Room Support', desc: 'Group clients into rooms for targeted broadcasting' },
  { title: 'Middleware System', desc: 'Chain handlers with custom logic using middleware pattern' },
  { title: 'Lifecycle Hooks', desc: 'Hook into connection, message, and room events' },
  { title: 'Pluggable Architecture', desc: 'Bring your own logger, metrics collector' },
  { title: 'Thread Safe', desc: 'All methods are safe for concurrent use' },
];

export default function Hero() {
  const [copied, setCopied] = useState(false);
  const { selectedVersion, getInstallCmd, minVersion } = useVersion();
  const installCmd = getInstallCmd(selectedVersion);
  const v110 = minVersion('v1.1.0');

  const filteredFeatures = useMemo(
    () => features.filter((f) => !f.minVersion || minVersion(f.minVersion)),
    [minVersion],
  );

  const handleCopy = () => {
    navigator.clipboard.writeText(installCmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section id="top" className="py-16 border-b border-border">
      <h1 className="text-4xl md:text-5xl font-bold text-text-heading mb-4">
        Production-ready Go WebSocket hub
      </h1>
      <p className="text-lg text-text-muted max-w-2xl mb-8">
        A reusable WebSocket connection management package for Go. Rooms, broadcasting,
        middleware, lifecycle hooks, metrics, rate limiting,{v110 ? ' multi-node scaling,' : ''} and more —
        with a pluggable, zero-business-logic architecture.
      </p>

      <div className="flex items-center gap-2 bg-bg-card border border-border rounded-lg px-4 py-3 max-w-lg mb-10">
        <span className="text-text-muted select-none">$</span>
        <code className="flex-1 text-sm font-mono text-accent">{installCmd}</code>
        <button
          onClick={handleCopy}
          className="text-xs text-text-muted hover:text-text px-2 py-1 rounded bg-overlay hover:bg-overlay-hover transition-colors"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredFeatures.map((f) => (
          <div key={f.title} className="bg-bg-card border border-border rounded-lg p-4">
            <h3 className="text-sm font-semibold text-text-heading mb-1">{f.title}</h3>
            <p className="text-xs text-text-muted">{f.desc}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
