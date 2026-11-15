import { OrbitControls } from "@react-three/drei";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import { memo, useEffect, useMemo, useRef, useState } from "react";
import * as THREE from "three";
import type { OrbitControls as OrbitControlsImpl } from "three-stdlib";
import { type City, type Place, placeScene, routePoints, toScene } from "../lib/geo";
import { cabPoses, isActive, useApp } from "../lib/store";
import { CityModel } from "./City";
import { Label } from "./Label";

const COLORS = { available: "#f6b21b", on_trip: "#22c7b8", mine: "#ffffff", pickup: "#3ddc84", drop: "#ff5d8f", idle: "#f6b21b" };

export function Scene() {
  const city = useApp((s) => s.city);
  return (
    <Canvas
      flat
      dpr={[1, 2]}
      camera={{ position: [-170, 360, 330], fov: 40, near: 1, far: 4000 }}
      gl={{ antialias: true, powerPreference: "high-performance" }}
    >
      <color attach="background" args={["#060a14"]} />
      <fog attach="fog" args={["#060a14", 620, 1500]} />
      <hemisphereLight args={["#9db9ff", "#0a0e18", 0.9]} />
      <directionalLight position={[-260, 420, 180]} intensity={1.1} color="#d5e2ff" />
      <directionalLight position={[300, 200, -260]} intensity={0.35} color="#ffb86b" />
      {city && (
        <>
          <CityModel city={city} />
          <Places city={city} />
          <Route city={city} />
          <Fleet city={city} />
          <Rig city={city} />
        </>
      )}
    </Canvas>
  );
}

// ---------- places ----------

function Places({ city }: { city: City }) {
  const pickupId = useApp((s) => s.pickupId);
  const dropId = useApp((s) => s.dropId);
  const trip = useApp((s) => s.trip);
  const signedIn = useApp((s) => !!s.user);
  const active = isActive(trip);

  return (
    <>
      {city.places.map((p) => {
        let role: "pickup" | "drop" | "idle" = "idle";
        if (active ? p.name === trip!.pickup_place : p.id === pickupId) role = "pickup";
        else if (active ? p.name === trip!.drop_place : p.id === dropId) role = "drop";
        return <Beacon key={p.id} place={p} role={role} interactive={signedIn && !active} dim={active && role === "idle"} />;
      })}
    </>
  );
}

const Beacon = memo(function Beacon({
  place,
  role,
  interactive,
  dim,
}: {
  place: Place;
  role: "pickup" | "drop" | "idle";
  interactive: boolean;
  dim: boolean;
}) {
  const [x, z] = placeScene(place);
  const [hover, setHover] = useState(false);
  const ring = useRef<THREE.Mesh>(null);
  const color = COLORS[role];
  const selected = role !== "idle";
  const height = selected ? 46 : 22;

  useFrame(({ clock }) => {
    if (!ring.current) return;
    const k = 1 + ((clock.elapsedTime * (selected ? 0.9 : 0.45) + x * 0.01) % 1) * (selected ? 2.2 : 1.2);
    ring.current.scale.set(k, k, 1);
    (ring.current.material as THREE.MeshBasicMaterial).opacity = (selected ? 0.9 : 0.45) * (1 - (k - 1) / 2.2);
  });

  return (
    <group position={[x, 0, z]}>
      <mesh position-y={height / 2}>
        <cylinderGeometry args={[selected ? 0.55 : 0.3, selected ? 0.55 : 0.3, height, 8]} />
        <meshBasicMaterial color={color} transparent opacity={dim ? 0.15 : selected ? 0.95 : 0.55} />
      </mesh>
      <mesh ref={ring} rotation-x={-Math.PI / 2} position-y={0.12}>
        <ringGeometry args={[3.2, 3.9, 40]} />
        <meshBasicMaterial color={color} transparent opacity={0.6} depthWrite={false} />
      </mesh>
      {/* generous invisible hit target */}
      <mesh
        position-y={height / 2}
        visible={false}
        onClick={(e) => {
          if (!interactive) return;
          e.stopPropagation();
          useApp.getState().selectPlace(place.id);
        }}
        onPointerOver={(e) => {
          if (!interactive) return;
          e.stopPropagation();
          setHover(true);
          document.body.style.cursor = "pointer";
        }}
        onPointerOut={() => {
          setHover(false);
          document.body.style.cursor = "";
        }}
      >
        <cylinderGeometry args={[7, 7, height + 6, 10]} />
      </mesh>
      {!dim && (
        <Label
          position={[0, height + 5, 0]}
          text={place.name}
          tag={role === "idle" ? undefined : role}
          color={role !== "idle" ? color : hover ? COLORS.idle : undefined}
          size={selected ? 1.25 : 1}
        />
      )}
    </group>
  );
});

// ---------- planned / active route ----------

function Route({ city }: { city: City }) {
  const pickupId = useApp((s) => s.pickupId);
  const dropId = useApp((s) => s.dropId);
  const trip = useApp((s) => s.trip);

  const segments = useMemo(() => {
    let from: Place | undefined;
    let to: Place | undefined;
    if (isActive(trip)) {
      from = city.places.find((p) => p.name === trip!.pickup_place);
      to = city.places.find((p) => p.name === trip!.drop_place);
    } else {
      from = city.places.find((p) => p.id === pickupId);
      to = city.places.find((p) => p.id === dropId);
    }
    if (!from || !to) return [];
    const pts = routePoints(city, from, to);
    return pts.slice(1).map((b, i) => {
      const a = pts[i];
      const len = Math.hypot(b[0] - a[0], b[1] - a[1]);
      const alongX = Math.abs(b[0] - a[0]) > Math.abs(b[1] - a[1]);
      return { key: `${a}-${b}`, pos: [(a[0] + b[0]) / 2, 0.25, (a[1] + b[1]) / 2] as const, size: alongX ? [len + 2, 0.2, 2] : [2, 0.2, len + 2] };
    });
  }, [city, pickupId, dropId, trip]);

  if (segments.length === 0) return null;
  return (
    <group>
      {segments.map((s) => (
        <mesh key={s.key} position={s.pos as unknown as THREE.Vector3Tuple}>
          <boxGeometry args={s.size as [number, number, number]} />
          <meshBasicMaterial color="#5cc8ff" transparent opacity={0.7} depthWrite={false} />
        </mesh>
      ))}
    </group>
  );
}

// ---------- fleet ----------

function Fleet({ city }: { city: City }) {
  const cabs = useApp((s) => s.cabs);
  const myCabId = useApp((s) => (isActive(s.trip) ? s.trip!.cab_id : null));
  return (
    <>
      {Object.values(cabs).map((c) => (
        <CabMesh key={c.id} id={c.id} city={city} status={c.status} mine={c.id === myCabId} plate={c.plate} />
      ))}
    </>
  );
}

const LANE_OFFSET = 2.3; // India drives on the left

/** Scene position for a cab pose, shifted into the left lane. */
function lanePosition(city: City, lat: number, lng: number, headingDeg: number, out: THREE.Vector3) {
  const [x, z] = toScene(city, lat, lng);
  const h = (headingDeg * Math.PI) / 180;
  return out.set(x - Math.cos(h) * LANE_OFFSET, 0, z - Math.sin(h) * LANE_OFFSET);
}

const CabMesh = memo(function CabMesh({
  id,
  city,
  status,
  mine,
  plate,
}: {
  id: string;
  city: City;
  status: string;
  mine: boolean;
  plate: string;
}) {
  const group = useRef<THREE.Group>(null);
  const halo = useRef<THREE.Mesh>(null);
  const target = useMemo(() => new THREE.Vector3(), []);
  const placed = useRef(false);
  const color = mine ? COLORS.mine : status === "on_trip" ? COLORS.on_trip : COLORS.available;

  useFrame(({ clock }, dt) => {
    const g = group.current;
    const pose = cabPoses.get(id);
    if (!g || !pose) return;
    lanePosition(city, pose.lat, pose.lng, pose.heading, target);
    const yaw = (-pose.heading * Math.PI) / 180;
    if (!placed.current || g.position.distanceToSquared(target) > 60 * 60) {
      // First sample, or a jump after reconnect: snap instead of gliding across the map.
      g.position.copy(target);
      g.rotation.y = yaw;
      placed.current = true;
    } else {
      // Updates arrive about once a second; ease towards the latest one.
      const k = 1 - Math.exp(-dt * 2.6);
      g.position.lerp(target, k);
      let d = yaw - g.rotation.y;
      d = Math.atan2(Math.sin(d), Math.cos(d));
      g.rotation.y += d * (1 - Math.exp(-dt * 7));
    }
    g.visible = true;
    if (halo.current) {
      const p = 1 + Math.sin(clock.elapsedTime * 4) * 0.12;
      halo.current.scale.set(p, p, 1);
    }
  });

  return (
    <group ref={group} visible={false} scale={1.55}>
      {/* soft ground glow so cabs read at a distance */}
      <mesh rotation-x={-Math.PI / 2} position-y={0.06}>
        <circleGeometry args={[3.6, 24]} />
        <meshBasicMaterial color={color} transparent opacity={0.22} depthWrite={false} />
      </mesh>
      {/* the model faces -Z (north) at yaw 0 */}
      <mesh position={[0, 0.95, 0]}>
        <boxGeometry args={[2.5, 1.0, 5.2]} />
        <meshStandardMaterial color={color} roughness={0.35} metalness={0.3} emissive={color} emissiveIntensity={0.55} />
      </mesh>
      <mesh position={[0, 1.8, 0.25]}>
        <boxGeometry args={[2.15, 0.85, 2.7]} />
        <meshStandardMaterial color="#0c1322" roughness={0.15} metalness={0.6} />
      </mesh>
      <mesh position={[0, 2.38, 0.25]}>
        <boxGeometry args={[1.0, 0.3, 0.5]} />
        <meshBasicMaterial color={status === "available" ? "#fff3b0" : "#ff7a59"} />
      </mesh>
      {[-0.85, 0.85].map((x) => (
        <group key={x}>
          <mesh position={[x, 0.95, -2.62]}>
            <boxGeometry args={[0.5, 0.3, 0.08]} />
            <meshBasicMaterial color="#fffbe6" />
          </mesh>
          <mesh position={[x, 0.95, 2.62]}>
            <boxGeometry args={[0.5, 0.25, 0.08]} />
            <meshBasicMaterial color="#ff3344" />
          </mesh>
        </group>
      ))}
      {[
        [-1.25, -1.6],
        [1.25, -1.6],
        [-1.25, 1.6],
        [1.25, 1.6],
      ].map(([x, z]) => (
        <mesh key={`${x},${z}`} position={[x, 0.45, z]} rotation-z={Math.PI / 2}>
          <cylinderGeometry args={[0.45, 0.45, 0.35, 10]} />
          <meshStandardMaterial color="#05070c" roughness={0.9} />
        </mesh>
      ))}
      {mine && (
        <>
          <mesh ref={halo} rotation-x={-Math.PI / 2} position-y={0.08}>
            <ringGeometry args={[4.4, 5.3, 40]} />
            <meshBasicMaterial color="#5cc8ff" transparent opacity={0.85} depthWrite={false} />
          </mesh>
          <Label position={[0, 7, 0]} text={plate} color="#5cc8ff" />
        </>
      )}
    </group>
  );
});

// ---------- camera ----------

function Rig({ city }: { city: City }) {
  const controls = useRef<OrbitControlsImpl>(null);
  const signedIn = useApp((s) => !!s.user);
  const cabId = useApp((s) => (isActive(s.trip) && s.followCab ? s.trip!.cab_id : null));
  const goal = useMemo(() => new THREE.Vector3(), []);
  const camera = useThree((s) => s.camera) as THREE.PerspectiveCamera;
  const size = useThree((s) => s.size);

  // Shift the projection so the city is centred in the area the panel does not cover:
  // right of the side panel on wide screens, above the bottom sheet on phones.
  useEffect(() => {
    const [dx, dy] = size.width > 820 ? [-194, 0] : [0, size.height * 0.24];
    camera.setViewOffset(size.width, size.height, dx, dy, size.width, size.height);
    return () => camera.clearViewOffset();
  }, [camera, size.width, size.height]);

  const limit = ((city.grid.size - 1) / 2) * city.grid.spacing_m * 0.1 + 40;

  useFrame((_, dt) => {
    const c = controls.current;
    if (!c) return;
    const pose = cabId ? cabPoses.get(cabId) : undefined;
    if (pose) {
      const [x, z] = toScene(city, pose.lat, pose.lng);
      c.target.lerp(goal.set(x, 0, z), 1 - Math.exp(-dt * 1.8));
    }
    // Keep the orbit centre over the city so the user cannot pan into the void.
    c.target.x = THREE.MathUtils.clamp(c.target.x, -limit, limit);
    c.target.z = THREE.MathUtils.clamp(c.target.z, -limit, limit);
    c.target.y = 0;
    c.update();
  });

  return (
    <OrbitControls
      ref={controls}
      makeDefault
      enableDamping
      dampingFactor={0.08}
      minDistance={45}
      maxDistance={820}
      minPolarAngle={0.15}
      maxPolarAngle={1.38}
      autoRotate={!signedIn}
      autoRotateSpeed={0.45}
    />
  );
}
