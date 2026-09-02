import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import type { ReactNode } from 'react';

import styles from './index.module.css';

type Feature = {
  readonly title: string;
  readonly description: string;
};

type Capability = {
  readonly capability: string;
};

type Stat = {
  readonly label: string;
  readonly value: string;
  readonly unit: string;
  readonly context: string;
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

// Everything wshub provides that a raw WebSocket library leaves to you. Kept in
// sync with the "Why wshub?" table in the repository README.
const CAPABILITIES = [
  { capability: 'Connection registry, rooms, broadcasting' },
  { capability: 'Backpressure & drop policies' },
  { capability: 'Graceful drain + shutdown' },
  { capability: 'Multi-node scaling (Redis / NATS)' },
  { capability: 'Rate limiting (connections, rooms, messages)' },
  { capability: 'Metrics + official Prometheus subpackage' },
  { capability: '/healthz and /readyz probes' },
  { capability: 'Lifecycle hooks & middleware chain' },
] as const satisfies readonly Capability[];

// Measured on an Intel i5-11400H @ 2.70GHz (12 cores), Go 1.27, Linux. Kept in
// sync with the Benchmarks section of the repository README.
const STATS = [
  {
    label: 'SendToClient',
    value: '105',
    unit: 'ns/op',
    context: '0 allocs, at 100K clients',
  },
  {
    label: 'Broadcast',
    value: '22.6',
    unit: 'ms',
    context: '0 allocs, to 100K clients',
  },
  {
    label: 'Handshakes',
    value: '36,891',
    unit: 'conn/s',
    context: 'measured over 10K connections',
  },
  {
    label: 'Fanout',
    value: '499K',
    unit: 'msg/s',
    context: '5K clients, one broadcaster',
  },
] as const satisfies readonly Stat[];

const INSTALL_COMMAND = 'go get github.com/KARTIKrocks/wshub';

const REPO_URL = 'https://github.com/KARTIKrocks/wshub';

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

function WhyWshub(): ReactNode {
  return (
    <section className={styles.section}>
      <div className="container">
        <h2 className={styles.sectionTitle}>Why wshub?</h2>
        <p className={styles.sectionLead}>
          A raw <code>gorilla/websocket</code> connection gets you a socket.
          Everything past that — the parts that turn “I can send a frame” into
          “I can run this in production” — is what wshub provides. It is not a
          replacement for a WebSocket protocol library; it is built on top of
          one.
        </p>

        <div className={styles.tableScroll}>
          <table className={styles.compare}>
            <thead>
              <tr>
                <th scope="col">Capability</th>
                <th scope="col">wshub</th>
                <th scope="col">Raw WebSocket library</th>
              </tr>
            </thead>
            <tbody>
              {CAPABILITIES.map(({ capability }) => (
                <tr key={capability}>
                  <th scope="row">{capability}</th>
                  <td>
                    <span className={styles.check} aria-hidden="true">
                      ✓
                    </span>
                    <span className={styles.srOnly}>Included</span>
                  </td>
                  <td className={styles.diy}>You build it</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function Performance(): ReactNode {
  return (
    <section className={styles.section}>
      <div className="container">
        <h2 className={styles.sectionTitle}>Performance</h2>
        <p className={styles.sectionLead}>
          Measured on an Intel i5-11400H (12 cores), Go 1.27, Linux — in-process
          benchmarks for dispatch cost, and end-to-end load tests over real
          WebSocket connections for the rest.
        </p>

        <div className={styles.stats}>
          {STATS.map((stat) => (
            <article key={stat.label} className={styles.stat}>
              <h3 className={styles.statLabel}>{stat.label}</h3>
              <p className={styles.statValue}>
                {stat.value}
                <span className={styles.statUnit}> {stat.unit}</span>
              </p>
              <p className={styles.statContext}>{stat.context}</p>
            </article>
          ))}
        </div>

        <p className={styles.sectionNote}>
          Reproduce them yourself with{' '}
          <code>go test -bench=. -benchmem ./...</code> and{' '}
          <code>make loadtest</code> — the{' '}
          <Link to={`${REPO_URL}#benchmarks`}>full benchmark tables</Link> list
          every figure and the exact flags behind it.
        </p>
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
        <WhyWshub />
        <Performance />
      </main>
    </Layout>
  );
}
