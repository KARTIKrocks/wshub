import { useState } from 'react';
import Navbar from './components/Navbar';
import Sidebar from './components/Sidebar';
import Hero from './components/Hero';
import GettingStarted from './content/getting-started';
import HubDocs from './content/hub';
import ClientDocs from './content/client';
import MessagesDocs from './content/messages';
import RoomsDocs from './content/rooms';
import MiddlewareDocs from './content/middleware';
import RouterDocs from './content/router';
import HooksDocs from './content/hooks';
import ConfigDocs from './content/config';
import LimitsDocs from './content/limits';
import MetricsDocs from './content/metrics';
import ErrorsDocs from './content/errors';

export default function App() {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="min-h-screen">
      <Navbar onMenuToggle={() => setMenuOpen((o) => !o)} menuOpen={menuOpen} />
      <Sidebar open={menuOpen} onClose={() => setMenuOpen(false)} />

      <main className="pt-16 md:pl-64">
        <div className="max-w-4xl mx-auto px-4 md:px-8 pb-20">
          <Hero />
          <GettingStarted />
          <HubDocs />
          <ClientDocs />
          <MessagesDocs />
          <RoomsDocs />
          <MiddlewareDocs />
          <RouterDocs />
          <HooksDocs />
          <ConfigDocs />
          <LimitsDocs />
          <MetricsDocs />
          <ErrorsDocs />

          <footer className="py-10 text-center text-sm text-text-muted border-t border-border mt-10">
            <p>
              wshub is open source under the{' '}
              <a
                href="https://github.com/KARTIKrocks/wshub/blob/main/LICENSE"
                className="text-primary hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                MIT License
              </a>
            </p>
          </footer>
        </div>
      </main>
    </div>
  );
}
