import { useEffect, useMemo } from "react";
import * as THREE from "three";

const FONT = '600 26px "Segoe UI Variable", "Segoe UI", system-ui, -apple-system, Roboto, sans-serif';
const TAG_FONT = '700 19px "Segoe UI Variable", "Segoe UI", system-ui, -apple-system, Roboto, sans-serif';
const H = 52;
const PAD = 20;

/**
 * A pill label drawn to a canvas and shown as a constant-size sprite. Living inside
 * the WebGL scene (instead of DOM overlays) means labels never cover the UI panels
 * and need no per-frame layout.
 */
export function Label({
  text,
  tag,
  color = "#3a4662",
  position,
  size = 1,
}: {
  text: string;
  tag?: string;
  color?: string;
  position: [number, number, number];
  size?: number;
}) {
  const { texture, aspect } = useMemo(() => {
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d")!;
    ctx.font = FONT;
    const textW = ctx.measureText(text).width;
    ctx.font = TAG_FONT;
    const tagText = tag?.toUpperCase();
    const tagW = tagText ? ctx.measureText(tagText).width + 12 : 0;
    const w = Math.ceil(textW + tagW + PAD * 2);
    canvas.width = w;
    canvas.height = H;

    ctx.beginPath();
    ctx.roundRect(1.5, 1.5, w - 3, H - 3, (H - 3) / 2);
    ctx.fillStyle = "rgba(8, 13, 24, 0.88)";
    ctx.fill();
    ctx.lineWidth = 2.5;
    ctx.strokeStyle = color;
    ctx.stroke();

    ctx.textBaseline = "middle";
    let x = PAD;
    if (tagText) {
      ctx.font = TAG_FONT;
      ctx.fillStyle = color;
      ctx.fillText(tagText, x, H / 2 + 1);
      x += tagW;
    }
    ctx.font = FONT;
    ctx.fillStyle = "#eef2fc";
    ctx.fillText(text, x, H / 2 + 1);

    const texture = new THREE.CanvasTexture(canvas);
    texture.colorSpace = THREE.SRGBColorSpace;
    texture.minFilter = THREE.LinearFilter;
    texture.generateMipmaps = false;
    return { texture, aspect: w / H };
  }, [text, tag, color]);

  useEffect(() => () => texture.dispose(), [texture]);

  // With sizeAttenuation off, scale is a fraction of the view at distance 1.
  const h = 0.021 * size;
  return (
    <sprite position={position} scale={[h * aspect, h, 1]} renderOrder={20}>
      <spriteMaterial map={texture} sizeAttenuation={false} depthTest={false} depthWrite={false} transparent toneMapped={false} />
    </sprite>
  );
}
