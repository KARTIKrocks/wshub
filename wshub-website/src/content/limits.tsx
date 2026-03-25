import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function LimitsDocs() {
  const { minVersion } = useVersion();
  const v110 = minVersion('v1.1.0');

  return (
    <ModuleSection
      id="limits"
      title="Limits"
      description="Control connections, rooms, and message rates to protect your server from abuse."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Maximum total connections',
        'Per-user connection limits (multi-device control)',
        'Per-client room limits',
        'Per-room client limits',
        'Per-client message rate limiting',
      ]}
    >
      {/* ── Connection Limits ── */}
      <h3 id="limits-connections" className="text-lg font-semibold text-text-heading mt-8 mb-2">Connection Limits</h3>
      <CodeBlock code={`limits := wshub.DefaultLimits().
    WithMaxConnections(10000).       // max total connections
    WithMaxConnectionsPerUser(5)     // max connections per user ID

hub := wshub.NewHub(
    wshub.WithLimits(limits),
)`} />

      {/* ── Room Limits ── */}
      <h3 id="limits-rooms" className="text-lg font-semibold text-text-heading mt-8 mb-2">Room Limits</h3>
      <CodeBlock code={`limits := wshub.DefaultLimits().
    WithMaxRoomsPerClient(10).   // max rooms a client can join
    WithMaxClientsPerRoom(100)   // max clients per room`} />

      {/* ── Rate Limiting ── */}
      <h3 id="limits-rate" className="text-lg font-semibold text-text-heading mt-8 mb-2">Rate Limiting</h3>
      <p className="text-text-muted mb-3">
        {v110
          ? 'Per-client message rate limiting using a token-bucket algorithm. Tokens refill at MaxMessageRate per second, capped at a burst of MaxMessageRate. This provides smoother throttling than fixed windows:'
          : 'Per-client message rate limiting with 1-second sliding windows:'}
      </p>
      <CodeBlock code={`limits := wshub.DefaultLimits().
    WithMaxMessageRate(100) // ${v110 ? '100 tokens/sec, burst of 100' : 'max 100 messages per second per client'}

// Complete limits example
limits := wshub.DefaultLimits().
    WithMaxConnections(10000).
    WithMaxConnectionsPerUser(5).
    WithMaxRoomsPerClient(10).
    WithMaxClientsPerRoom(100).
    WithMaxMessageRate(100)

hub := wshub.NewHub(
    wshub.WithLimits(limits),
)`} />
    </ModuleSection>
  );
}
