import { useEffect, useState } from 'react';

// useHashRoute is a 10-line "router" — we don't need a real one for two
// static pages, and avoiding react-router keeps the bundle ~30KB
// smaller. Returns the current hash route ("home" | "privacy" | "terms")
// and re-renders the host component whenever it changes.
export type Route = 'home' | 'privacy' | 'terms';

export function useHashRoute(): Route {
  const [route, setRoute] = useState<Route>(parse(window.location.hash));

  useEffect(() => {
    const update = () => setRoute(parse(window.location.hash));
    window.addEventListener('hashchange', update);
    return () => window.removeEventListener('hashchange', update);
  }, []);

  return route;
}

function parse(hash: string): Route {
  switch (hash.replace(/^#\/?/, '').toLowerCase()) {
    case 'privacy':
      return 'privacy';
    case 'terms':
      return 'terms';
    default:
      return 'home';
  }
}
