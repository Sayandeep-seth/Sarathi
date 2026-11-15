import { useLayoutEffect, useMemo, useRef } from "react";
import * as THREE from "three";
import { type City, SCALE, mulberry32 } from "../lib/geo";

const ROAD_W = 10;
const SIDEWALK = 2;

/** Ground, road grid, lane dashes, procedural skyline and parks. Static: built once. */
export function CityModel({ city }: { city: City }) {
  const n = city.grid.size;
  const s = city.grid.spacing_m * SCALE;
  const half = ((n - 1) / 2) * s;
  const extent = half * 2 + ROAD_W;

  const lines = useMemo(() => Array.from({ length: n }, (_, i) => -half + i * s), [n, s, half]);

  return (
    <group>
      <mesh rotation-x={-Math.PI / 2} position-y={-0.2}>
        <planeGeometry args={[4000, 4000]} />
        <meshStandardMaterial color="#080d18" roughness={1} />
      </mesh>
      {/* district slab under the grid */}
      <mesh rotation-x={-Math.PI / 2} position-y={-0.1}>
        <planeGeometry args={[extent + 30, extent + 30]} />
        <meshStandardMaterial color="#0b111e" roughness={1} />
      </mesh>
      {lines.map((p) => (
        <group key={p}>
          <mesh rotation-x={-Math.PI / 2} position={[0, 0, p]}>
            <planeGeometry args={[extent, ROAD_W]} />
            <meshStandardMaterial color="#232f47" roughness={0.9} />
          </mesh>
          <mesh rotation-x={-Math.PI / 2} position={[p, 0.01, 0]}>
            <planeGeometry args={[ROAD_W, extent]} />
            <meshStandardMaterial color="#232f47" roughness={0.9} />
          </mesh>
        </group>
      ))}
      <LaneDashes lines={lines} half={half} spacing={s} />
      <Blocks city={city} />
    </group>
  );
}

function LaneDashes({ lines, half, spacing }: { lines: number[]; half: number; spacing: number }) {
  const ref = useRef<THREE.InstancedMesh>(null);
  const perBlock = 6;
  const count = lines.length * (lines.length - 1) * perBlock * 2;

  useLayoutEffect(() => {
    const mesh = ref.current!;
    const m = new THREE.Matrix4();
    const q = new THREE.Quaternion();
    const qTurn = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), Math.PI / 2);
    const scale = new THREE.Vector3(1, 1, 1);
    let k = 0;
    for (const p of lines) {
      for (let b = 0; b < lines.length - 1; b++) {
        for (let d = 0; d < perBlock; d++) {
          // keep dashes out of the intersections
          const along = -half + b * spacing + ROAD_W / 2 + ((d + 0.5) * (spacing - ROAD_W)) / perBlock;
          m.compose(new THREE.Vector3(along, 0.03, p), q, scale);
          mesh.setMatrixAt(k++, m);
          m.compose(new THREE.Vector3(p, 0.03, along), qTurn, scale);
          mesh.setMatrixAt(k++, m);
        }
      }
    }
    mesh.instanceMatrix.needsUpdate = true;
  }, [lines, half, spacing]);

  return (
    <instancedMesh ref={ref} args={[undefined, undefined, count]} frustumCulled={false}>
      <boxGeometry args={[2.6, 0.02, 0.22]} />
      <meshBasicMaterial color="#6b7896" />
    </instancedMesh>
  );
}

/** Adds procedurally lit windows to building sides, computed from world position. */
function useWindowMaterial() {
  return useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({ color: "#ffffff", roughness: 0.75, metalness: 0.15 });
    mat.onBeforeCompile = (shader) => {
      shader.vertexShader = shader.vertexShader
        .replace("#include <common>", "#include <common>\nvarying vec3 vWPos;\nvarying vec3 vWNrm;")
        .replace(
          "#include <begin_vertex>",
          `#include <begin_vertex>
          #ifdef USE_INSTANCING
            mat4 wm = modelMatrix * instanceMatrix;
          #else
            mat4 wm = modelMatrix;
          #endif
          vWPos = (wm * vec4(transformed, 1.0)).xyz;
          vWNrm = normalize(mat3(wm) * normal);`,
        );
      shader.fragmentShader = shader.fragmentShader
        .replace(
          "#include <common>",
          `#include <common>
          varying vec3 vWPos;
          varying vec3 vWNrm;
          float hash21(vec2 p) {
            p = fract(p * vec2(123.34, 456.21));
            p += dot(p, p + 45.32);
            return fract(p.x * p.y);
          }`,
        )
        .replace(
          "#include <emissivemap_fragment>",
          `#include <emissivemap_fragment>
          if (abs(vWNrm.y) < 0.5) {
            float u = abs(vWNrm.x) > 0.5 ? vWPos.z : vWPos.x;
            vec2 cell = vec2(u / 1.7, vWPos.y / 2.3);
            vec2 id = floor(cell);
            vec2 f = fract(cell);
            float lit = step(0.7, hash21(id + floor(vWNrm.xz * 7.0)));
            float mask = step(0.22, f.x) * step(f.x, 0.78) * step(0.3, f.y) * step(f.y, 0.78);
            vec3 tint = mix(vec3(1.0, 0.76, 0.42), vec3(0.5, 0.78, 1.0), step(0.8, hash21(id * 1.7)));
            totalEmissiveRadiance += tint * lit * mask * 0.7;
          }`,
        );
    };
    return mat;
  }, []);
}

function Blocks({ city }: { city: City }) {
  const buildings = useRef<THREE.InstancedMesh>(null);
  const trees = useRef<THREE.InstancedMesh>(null);
  const trunks = useRef<THREE.InstancedMesh>(null);
  const material = useWindowMaterial();

  const layout = useMemo(() => {
    const n = city.grid.size;
    const s = city.grid.spacing_m * SCALE;
    const half = ((n - 1) / 2) * s;
    const rand = mulberry32(20261115);
    const nodeHalf = (n - 1) / 2;

    // The block north-east of a park place becomes a park.
    const parks = new Set<string>();
    for (const p of city.places) {
      if (p.kind !== "park") continue;
      const i = Math.min(n - 2, Math.round(p.x / city.grid.spacing_m + nodeHalf));
      const j = Math.min(n - 2, Math.round(p.y / city.grid.spacing_m + nodeHalf));
      parks.add(`${i},${j}`);
    }

    const palette = ["#27344d", "#2d3a55", "#222d44", "#343f59", "#1f2a40", "#3a4663"].map((c) => new THREE.Color(c));
    const b: { x: number; z: number; w: number; d: number; h: number; color: THREE.Color }[] = [];
    const t: { x: number; z: number; h: number }[] = [];
    const parkPads: { x: number; z: number; size: number }[] = [];
    const inner = s - ROAD_W - SIDEWALK * 2;
    const lots = 3;
    const lot = inner / lots;

    for (let i = 0; i < n - 1; i++) {
      for (let j = 0; j < n - 1; j++) {
        const cx = -half + (i + 0.5) * s;
        const cz = -(-half + (j + 0.5) * s);
        if (parks.has(`${i},${j}`)) {
          parkPads.push({ x: cx, z: cz, size: inner });
          for (let k = 0; k < 22; k++) {
            t.push({ x: cx + (rand() - 0.5) * (inner - 4), z: cz + (rand() - 0.5) * (inner - 4), h: 3 + rand() * 3.5 });
          }
          continue;
        }
        // Taller towards the centre: a recognisable downtown.
        const centre = 1 - Math.min(1, Math.hypot(cx, cz) / (half * 1.15));
        for (let a = 0; a < lots; a++) {
          for (let c = 0; c < lots; c++) {
            if (rand() < 0.08) continue;
            const w = lot * (0.55 + rand() * 0.27);
            const d = lot * (0.55 + rand() * 0.27);
            // Kept low enough that roads and cabs stay visible from the default camera.
            const h = 3.5 + rand() ** 2 * (7 + centre * 30) + centre * 3;
            b.push({
              x: cx - inner / 2 + (a + 0.5) * lot,
              z: cz - inner / 2 + (c + 0.5) * lot,
              w,
              d,
              h,
              color: palette[Math.floor(rand() * palette.length)],
            });
          }
        }
      }
    }
    return { b, t, parkPads };
  }, [city]);

  useLayoutEffect(() => {
    const m = new THREE.Matrix4();
    const q = new THREE.Quaternion();
    const mesh = buildings.current!;
    layout.b.forEach((x, k) => {
      m.compose(new THREE.Vector3(x.x, x.h / 2, x.z), q, new THREE.Vector3(x.w, x.h, x.d));
      mesh.setMatrixAt(k, m);
      mesh.setColorAt(k, x.color);
    });
    mesh.instanceMatrix.needsUpdate = true;
    if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;

    layout.t.forEach((x, k) => {
      m.compose(new THREE.Vector3(x.x, x.h * 0.62 + 1, x.z), q, new THREE.Vector3(1, x.h, 1));
      trees.current!.setMatrixAt(k, m);
      m.compose(new THREE.Vector3(x.x, 0.7, x.z), q, new THREE.Vector3(1, 1, 1));
      trunks.current!.setMatrixAt(k, m);
    });
    trees.current!.instanceMatrix.needsUpdate = true;
    trunks.current!.instanceMatrix.needsUpdate = true;
  }, [layout]);

  return (
    <group>
      <instancedMesh ref={buildings} args={[undefined, undefined, layout.b.length]} material={material} frustumCulled={false}>
        <boxGeometry args={[1, 1, 1]} />
      </instancedMesh>
      {layout.parkPads.map((p) => (
        <mesh key={`${p.x},${p.z}`} rotation-x={-Math.PI / 2} position={[p.x, 0.05, p.z]}>
          <planeGeometry args={[p.size, p.size]} />
          <meshStandardMaterial color="#12301f" roughness={1} />
        </mesh>
      ))}
      <instancedMesh ref={trees} args={[undefined, undefined, layout.t.length]} frustumCulled={false}>
        <coneGeometry args={[1.5, 1, 7]} />
        <meshStandardMaterial color="#1f6b3f" roughness={0.9} flatShading />
      </instancedMesh>
      <instancedMesh ref={trunks} args={[undefined, undefined, layout.t.length]} frustumCulled={false}>
        <cylinderGeometry args={[0.25, 0.3, 1.4, 5]} />
        <meshStandardMaterial color="#3b2a1c" roughness={1} />
      </instancedMesh>
    </group>
  );
}
