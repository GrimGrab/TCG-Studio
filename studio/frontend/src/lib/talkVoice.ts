// A customer's talking sound, as the mod's Runtime/Customers/TalkSounds.cs makes it, for the preview on the Customers page. The way
// animalese.js does it (its letter recordings, CC BY 4.0): a whole line is rendered into one sound, every character getting the same
// short slot, read from the start of its letter's recording. Pitch is how fast the recording is read. Change both together.
import lettersUrl from '../assets/talk/letters.wav?url';

export type Voice = { pitch: number; tone: number };

/** A voice at every middle. */
export const PLAIN_VOICE: Voice = { pitch: 1, tone: 0.5 };
export type Mood = 'neutral' | 'happy' | 'angry' | 'sad' | 'excited';

const RATE = 44100;
const WAV_HEADER = 44;
export const LETTER_SAMPLES = 6615; // 0.15 s per letter in the recording
export const SLOT_SAMPLES = 3307; // 0.075 s per character spoken
const DIGIT_LETTERS = 'zwttffssen'; // a digit sounds like the start of its name

const MOODS: Record<Mood, { pitch: number; speed: number; loud: number; swing: number; fall: number }> = {
  neutral: { pitch: 1, speed: 1, loud: 1, swing: 0.03, fall: 0.05 },
  happy: { pitch: 1.08, speed: 1.05, loud: 1, swing: 0.08, fall: 0 },
  angry: { pitch: 0.9, speed: 1.12, loud: 1.3, swing: 0.06, fall: 0.1 },
  sad: { pitch: 0.82, speed: 0.82, loud: 0.7, swing: 0.02, fall: 0.15 },
  excited: { pitch: 1.15, speed: 1.12, loud: 1.25, swing: 0.1, fall: -0.08 },
};

export const MOOD_NAMES = Object.keys(MOODS) as Mood[];

/** The voice of someone without a saved one, the same every visit: everything from their id, the pitch within their body's range. */
export function automaticVoice(id: string, female: boolean): Voice {
  const roll = (salt: string) => {
    let h = 2166136261;
    for (const c of salt + id) h = Math.imul(h ^ c.charCodeAt(0), 16777619) >>> 0;
    return (h % 1000) / 1000;
  };
  return {
    pitch: (female ? 1.25 : 0.8) + roll('') * (female ? 0.45 : 0.25),
    tone: 0.3 + roll('t') * 0.4,
  };
}

/** A line as it might be said: its {tokens} filled in with sample words. */
export function sampleLine(line: string): string {
  return line.replace(/\{(\w+)\}/g, (_, k: string) =>
    (({ name: 'Sam', shop: 'Card Shop', card: 'Black Lotus', card2: 'Lightning Bolt', rival: 'Alex', about: 'Jamie', price: '20 dollars', when: 'yesterday' }) as Record<string, string>)[k] ?? k);
}

const isAlnum = (c: string) => /[\p{L}\p{N}]/u.test(c);

/** The line as samples. letters = the whole recording file. */
export function renderLine(text: string, voice: Voice, moodName: Mood, letters: Uint8Array): Float32Array {
  const mood = MOODS[moodName];
  const chars = text.split('');
  const slot = Math.floor(SLOT_SAMPLES / mood.speed);
  const spoken = chars.filter(isAlnum);
  const upper = spoken.filter((c) => c !== c.toLowerCase()).length;
  const end = text.trimEnd();
  const question = end.endsWith('?') || end.endsWith('?!');
  const shout = end.endsWith('!') || (spoken.length > 3 && upper * 2 > spoken.length);
  const data = new Float32Array(chars.length * slot);
  let count = 0;
  chars.forEach((raw, c) => {
    if (!isAlnum(raw)) return; // silence
    let ch = raw.toLowerCase();
    if (ch >= '0' && ch <= '9') ch = DIGIT_LETTERS[+ch];
    else if (ch < 'a' || ch > 'z') ch = 'aeiou'[ch.charCodeAt(0) % 5]; // other alphabets
    const progress = spoken.length > 1 ? count / (spoken.length - 1) : 0;
    let tune = 1 - mood.fall * progress + mood.swing * Math.sin(count * 0.6) + (Math.random() - 0.5) * 0.04;
    if (question && progress > 0.7) tune *= 1 + (progress - 0.7) * 0.7;
    if (shout && progress > 0.8) tune *= 1.05;
    count++;
    const rate = voice.pitch * mood.pitch * tune;
    const start = LETTER_SAMPLES * (ch.charCodeAt(0) - 97);
    for (let i = 0; i < slot; i++) {
      const at = Math.floor(i * rate);
      if (at >= LETTER_SAMPLES) break;
      data[c * slot + i] = (letters[WAV_HEADER + start + at] - 128) / 128;
    }
  });
  applyTone(data, voice.tone);
  return data;
}

/** The middle leaves the recording alone; below it the sound gets rounder, above it crisper. Keeps its loudness. */
function applyTone(d: Float32Array, tone: number): void {
  if (Math.abs(tone - 0.5) < 0.02) return;
  const peak = () => d.reduce((m, v) => Math.max(m, Math.abs(v)), 0.001);
  const before = peak();
  if (tone < 0.5) {
    const alpha = 0.08 + 0.92 * Math.max(tone, 0) * 2;
    for (let pass = 0; pass < 2; pass++) {
      let last = 0;
      for (let i = 0; i < d.length; i++) d[i] = last += alpha * (d[i] - last);
    }
  } else {
    const push = (Math.min(tone, 1) - 0.5) * 3.6;
    let last = 0;
    for (let i = 0; i < d.length; i++) {
      const v = d[i];
      d[i] = (1 + push) * v - push * last;
      last = v;
    }
  }
  const scale = before / peak();
  for (let i = 0; i < d.length; i++) d[i] *= scale;
}

let ctx: AudioContext | null = null;
let current: AudioBufferSourceNode | null = null;
let recording: Promise<Uint8Array> | null = null;

/** Speaks text in this voice and mood, replacing what was playing. */
export async function previewVoice(voice: Voice, text: string, moodName: Mood = 'neutral'): Promise<void> {
  ctx ??= new AudioContext();
  recording ??= fetch(lettersUrl).then((r) => r.arrayBuffer()).then((b) => new Uint8Array(b));
  const data = renderLine(text, voice, moodName, await recording);
  if (!data.length) return;
  const buffer = ctx.createBuffer(1, data.length, RATE);
  buffer.getChannelData(0).set(data);
  current?.stop();
  current = ctx.createBufferSource();
  current.buffer = buffer;
  const g = ctx.createGain();
  g.gain.value = Math.min(1, 0.6 * MOODS[moodName].loud);
  current.connect(g).connect(ctx.destination);
  current.start();
}
