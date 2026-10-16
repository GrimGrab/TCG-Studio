// Pages with unsaved changes ask before the sidebar takes the player elsewhere. A page registers its guard while it is
// shown: $effect(() => leaveGuard(() => dirty, 'Leave … without saving?')) (the returned function unregisters it).
import { ask } from './api';

type Guard = { dirty: () => boolean; message: string };
const guards: Guard[] = [];

export function leaveGuard(dirty: () => boolean, message: string): () => void {
  const g = { dirty, message };
  guards.push(g);
  return () => {
    const i = guards.indexOf(g);
    if (i >= 0) guards.splice(i, 1);
  };
}

/** True when nothing unsaved is shown, or the player agreed to lose it. */
export async function canLeave(): Promise<boolean> {
  for (const g of [...guards].reverse()) {
    if (g.dirty() && !(await ask(g.message))) return false;
  }
  return true;
}
