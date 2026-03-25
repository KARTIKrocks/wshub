import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { VersionContext, type Release } from '../context/VersionContext';

function getUrlVersion(): string | null {
  const params = new URLSearchParams(window.location.search);
  return params.get('v');
}

function updateUrlVersion(version: string, latestVersion: string) {
  const url = new URL(window.location.href);
  if (version === latestVersion) {
    url.searchParams.delete('v');
  } else {
    url.searchParams.set('v', version);
  }
  history.replaceState(null, '', url.toString());
}

export default function VersionProvider({ children }: { children: ReactNode }) {
  const [releases, setReleases] = useState<Release[]>([]);
  const [selectedVersion, setSelectedVersionState] = useState(() => getUrlVersion() ?? 'v1.1.0');
  const [loading, setLoading] = useState(true);
  const [latestVersion, setLatestVersion] = useState('v1.1.0');

  useEffect(() => {
    const controller = new AbortController();
    fetch('https://api.github.com/repos/KARTIKrocks/wshub/releases', {
      signal: controller.signal,
      headers: { Accept: 'application/vnd.github.v3+json' },
    })
      .then((r) => r.json())
      .then((data: Release[]) => {
        if (Array.isArray(data)) {
          const stable = data.filter((r) => !r.prerelease);
          setReleases(stable);
          if (stable.length > 0) {
            const latest = stable[0].tag_name;
            setLatestVersion(latest);

            const urlVersion = getUrlVersion();
            if (urlVersion && stable.some((r) => r.tag_name === urlVersion)) {
              setSelectedVersionState(urlVersion);
            } else {
              setSelectedVersionState(latest);
            }
          }
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));

    return () => controller.abort();
  }, []);

  const setSelectedVersion = useCallback(
    (version: string) => {
      setSelectedVersionState(version);
      updateUrlVersion(version, latestVersion);
    },
    [latestVersion],
  );

  // Keep URL in sync when latestVersion resolves after initial load
  useEffect(() => {
    if (!loading) {
      updateUrlVersion(selectedVersion, latestVersion);
    }
  }, [loading, latestVersion, selectedVersion]);

  const isLatest = selectedVersion === latestVersion;

  function getReleaseUrl(version: string): string {
    const release = releases.find((r) => r.tag_name === version);
    return release?.html_url ?? `https://github.com/KARTIKrocks/wshub/releases/tag/${version}`;
  }

  function getInstallCmd(version: string): string {
    if (version === latestVersion) {
      return 'go get github.com/KARTIKrocks/wshub';
    }
    return `go get github.com/KARTIKrocks/wshub@${version}`;
  }

  return (
    <VersionContext.Provider
      value={{
        releases,
        selectedVersion,
        latestVersion,
        setSelectedVersion,
        loading,
        isLatest,
        getReleaseUrl,
        getInstallCmd,
      }}
    >
      {children}
    </VersionContext.Provider>
  );
}
