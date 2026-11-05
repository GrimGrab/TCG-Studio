// Conversions that rewrite a design when it opens (saved with the next save). The list of every migration, and when each can
// go, is internal/migrate/registry.go.
import { layerId, wholeFaceFill, type Layer, type Model } from './accessoryArt';

// migration:editor-face-fills
/** Face-net models: whole-face colours as colour rectangles that look the same — one the size of each face they cover
 *  (an all-faces colour: one per face, named after it); its gradient still runs down the face. null when there are none. */
export function faceFillsToRects(m: Model, layers: Layer[]): Layer[] | null {
  if (m.projected || !layers.some(wholeFaceFill)) return null;
  return layers.flatMap((l) => {
    if (!wholeFaceFill(l)) return [l];
    const faces = m.faces.filter((f) => !l.face || f.id === l.face);
    return faces.map((f): Layer => ({
      ...l, id: faces.length === 1 ? l.id : layerId(), name: faces.length === 1 ? l.name : `${l.name} – ${f.label}`, face: undefined,
      x: f.net[0] + f.net[2] / 2, y: f.net[1] + f.net[3] / 2, w: f.net[2], h: f.net[3], rot: 0,
    }));
  });
}
