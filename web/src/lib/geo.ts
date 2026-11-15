// Client-side mirror of backend/internal/geo. The server stays authoritative for
// distance and fare; this is only used to draw the scene and the planned route.

export interface Place {
  id: string;
  name: string;
  kind: string;
  lat: number;
  lng: number;
  x: number; // metres east of origin
  y: number; // metres north of origin
}

export interface City {
  name: string;
  origin: { lat: number; lng: number };
  grid: { size: number; spacing_m: number };
  places: Place[];
}

/** Scene units per metre. 1 unit = 10 m keeps the whole grid within ~450 units. */
export const SCALE = 0.1;

const M_PER_DEG = 111_320;

/** WGS84 -> scene coordinates. North is -Z so the map reads like a map from above. */
export function toScene(city: City, lat: number, lng: number): [number, number] {
  const east = (lng - city.origin.lng) * M_PER_DEG * Math.cos((city.origin.lat * Math.PI) / 180);
  const north = (lat - city.origin.lat) * M_PER_DEG;
  return [east * SCALE, -north * SCALE];
}

export function placeScene(p: Place): [number, number] {
  return [p.x * SCALE, -p.y * SCALE];
}

interface Node {
  i: number;
  j: number;
}

function nodeOf(city: City, p: Place): Node {
  const half = (city.grid.size - 1) / 2;
  return { i: Math.round(p.x / city.grid.spacing_m + half), j: Math.round(p.y / city.grid.spacing_m + half) };
}

/** Same L-shaped rule as geo.Route on the server, returned as scene points. */
export function routePoints(city: City, from: Place, to: Place): [number, number][] {
  const a = nodeOf(city, from);
  const b = nodeOf(city, to);
  const half = (city.grid.size - 1) / 2;
  const s = city.grid.spacing_m * SCALE;
  const pt = (n: Node): [number, number] => [(n.i - half) * s, -(n.j - half) * s];
  const corner: Node = (a.i + a.j + b.i + b.j) % 2 === 0 ? { i: b.i, j: a.j } : { i: a.i, j: b.j };
  const pts = [pt(a), pt(corner), pt(b)];
  return pts.filter((p, k) => k === 0 || p[0] !== pts[k - 1][0] || p[1] !== pts[k - 1][1]);
}

/** Driving distance in metres between two scene points on the grid (Manhattan). */
export function gridDistanceM(a: [number, number], b: [number, number]): number {
  return (Math.abs(a[0] - b[0]) + Math.abs(a[1] - b[1])) / SCALE;
}

/** Deterministic PRNG so the skyline is identical on every load. */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
