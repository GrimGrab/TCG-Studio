import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { fullViewOpen, fullWindow } from './maximize';

// Just enough of the browser for fullWindow (tests run in Node): the window's keydown listeners, an element, a placeholder.
const listeners: ((e: any) => void)[] = [];
beforeEach(() => {
  listeners.length = 0;
  Object.assign(globalThis, {
    window: { addEventListener: (_: string, f: any) => listeners.push(f), removeEventListener: () => {} },
    document: { createElement: () => ({ style: {}, remove() {} }) },
    getComputedStyle: () => ({ flex: '' }),
    requestAnimationFrame: (f: () => void) => f(),
  });
});
afterEach(() => { for (const k of ['window', 'document', 'getComputedStyle', 'requestAnimationFrame']) delete (globalThis as any)[k]; });

const element = () => ({
  style: {}, parentElement: { insertBefore() {}, parentElement: null, scrollHeight: 0, clientHeight: 0, scrollWidth: 0, clientWidth: 0 },
  getBoundingClientRect: () => ({ width: 10, height: 10 }), getAttribute: () => null, setAttribute() {}, removeAttribute() {},
}) as any;
const esc = (timeStamp: number) => ({ key: 'Escape', timeStamp, defaultPrevented: false, target: null, preventDefault() {} }) as unknown as KeyboardEvent;

describe('full view and Esc', () => {
  it('lets one Esc leave the full view only, whichever listener runs first', () => {
    const fw = fullWindow(element());
    fw.set(true);
    const first = esc(5);
    expect(fullViewOpen(first)).toBe(true); // a page's own listener running before the full view's: skips
    listeners.forEach((f) => f(first)); // the full view's listener closes it
    expect(fw.on).toBe(false);
    expect(fullViewOpen(first)).toBe(true); // a page's listener running after, for the same key press: still skips
    expect(fullViewOpen(esc(6))).toBe(false); // the next Esc is the page's again
    fw.destroy();
  });
  it('counts every full view open, and a destroyed one as closed', () => {
    const a = fullWindow(element()), b = fullWindow(element());
    a.set(true); b.set(true);
    a.set(false);
    expect(fullViewOpen()).toBe(true);
    b.destroy();
    expect(fullViewOpen()).toBe(false);
    a.destroy();
  });
});
