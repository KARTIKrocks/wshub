import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import type { ReactNode } from 'react';

import styles from './index.module.css';

type Feature = {
  readonly title: string;
  readonly description: string;
};

const FEATURES = [
  {
    title: 'Production Ready',
    description: 'Proper concurrency, graceful shutdown, error handling',
  },
  {
    title: 'Multi-Node Support',
    description:
      'Scale horizontally with Redis or NATS adapters and presence gossip',
  },
  {
    title: 'Room Support',
    description: 'Group clients into rooms for targeted broadcasting',
  },
  {
    title: 'Middleware System',
    description:
      'Chain handlers with custom logic using the middleware pattern',
  },
  {
    title: 'Lifecycle Hooks',
    description: 'Hook into connection, message, and room events',
  },
  {
    title: 'Pluggable Architecture',
    description: 'Bring your own logger and metrics collector',
  },
  {
    title: 'Thread Safe',
    description: 'All methods are safe for concurrent use',
  },
  {
    title: 'Zero-Alloc JSON',
    description:
      'Pre-serialized JSON API skips marshaling — 0 allocs, ~35 ns per send',
  },
  {
    title: 'Graceful Drain',
    description:
      'Zero-downtime deploys with connection draining, idle reaping, and hub state inspection',
  },
  {
    title: 'Write Coalescing',
    description:
      'Batch queued text messages into a single WebSocket frame, reducing syscalls under high throughput',
  },
  {
    title: 'Health Probes',
    description:
      'Drop-in HTTP handlers for Kubernetes /healthz and /readyz with live state, uptime, and client count',
  },
  {
    title: 'Prometheus Metrics',
    description:
      'Official drop-in MetricsCollector subpackage with 12 metrics covering connections, messages, latency, and rooms',
  },
] as const satisfies readonly Feature[];

const INSTALL_COMMAND = 'go get github.com/KARTIKrocks/wshub';

function Hero(): ReactNode {
  return (
    <header className={styles.hero}>
      <div className="container">
        <h1 className={styles.title}>Production-ready Go WebSocket hub</h1>
        <p className={styles.subtitle}>
          A reusable WebSocket connection management package for Go. Rooms,
          broadcasting, middleware, lifecycle hooks, metrics, rate limiting,
          multi-node scaling, and more — with a pluggable, zero-business-logic
          architecture.
        </p>

        <div className={styles.buttons}>
          <Link
            className="button button--primary button--lg"
            to="/docs/getting-started">
            Get Started
          </Link>
          <Link
            className="button button--secondary button--lg"
            to="https://pkg.go.dev/github.com/KARTIKrocks/wshub">
            API Reference
          </Link>
        </div>

        <div className={styles.install}>
          <span className={styles.prompt} aria-hidden="true">
            $
          </span>
          <code>{INSTALL_COMMAND}</code>
        </div>
      </div>
    </header>
  );
}

function Features(): ReactNode {
  return (
    <section className="container" aria-label="Features">
      <div className={styles.features}>
        {FEATURES.map((feature) => (
          <article key={feature.title} className={styles.card}>
            <h2>{feature.title}</h2>
            <p>{feature.description}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const { siteConfig } = useDocusaurusContext();

  return (
    <Layout
      title={siteConfig.tagline}
      description="A production-ready, scalable WebSocket package for Go with rooms, broadcasting, multi-node clustering, middleware, hooks, and extensibility.">
      <Hero />
      <main>
        <Features />
      </main>
    </Layout>
  );
}
