import { describe, expect, it } from "vitest";
import { bytes, rupees } from "./format";
import { type City, gridDistanceM, placeScene, routePoints, toScene } from "./geo";

const place = (id: string, i: number, j: number) => ({
  id, name: id, kind: "x", lat: 12.9716 + ((j - 5) * 450) / 111320,
  lng: 77.5946 + ((i - 5) * 450) / (111320 * Math.cos((12.9716 * Math.PI) / 180)), x: (i - 5) * 450, y: (j - 5) * 450,
});
const city: City = {
  name: "t", origin: { lat: 12.9716, lng: 77.5946 }, grid: { size: 11, spacing_m: 450 },
  places: [place("a", 5, 5), place("b", 8, 6), place("c", 5, 10)],
};

describe("geo", () => {
  it("projects lat/lng to the same scene point as grid metres", () => {
    for (const p of city.places) {
      const [x, z] = toScene(city, p.lat, p.lng);
      const [px, pz] = placeScene(p);
      expect(x).toBeCloseTo(px, 3);
      expect(z).toBeCloseTo(pz, 3);
    }
  });
  it("draws an L-shaped route whose length equals the Manhattan distance", () => {
    const [a, b, c] = city.places;
    const pts = routePoints(city, a, b);
    expect(pts).toHaveLength(3);
    let len = 0;
    for (let i = 1; i < pts.length; i++) len += gridDistanceM(pts[i - 1], pts[i]);
    expect(len).toBeCloseTo(4 * 450, 3);
    // even parity (5+5+8+6): the server walks east-west first, so the corner keeps a's row
    expect(pts[1][1]).toBeCloseTo(pts[0][1]);
    expect(routePoints(city, a, c)).toHaveLength(2); // straight line has no corner
  });
});

describe("format", () => {
  it("uses binary units and paise", () => {
    expect(bytes(2 ** 30)).toBe("1.00 GiB");
    expect(bytes(500 * 2 ** 20)).toBe("500.0 MiB");
    expect(rupees(8080)).toContain("80.80");
  });
});
