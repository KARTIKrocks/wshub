import { useContext } from 'react';
import { VersionContext, type VersionContextValue } from '../context/VersionContext';

export function useVersion(): VersionContextValue {
  const ctx = useContext(VersionContext);
  if (!ctx) throw new Error('useVersion must be used within VersionProvider');
  return ctx;
}
