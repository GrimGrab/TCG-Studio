import { describe, expect, it } from 'vitest';
import { animatableBones, euler, invAffine, mul4, qmul, skin, skinMatrices, trs, type Bone } from './skeleton';

const close = (a: ArrayLike<number>, b: ArrayLike<number>, eps = 1e-5) => {
  expect(a.length).toBe(b.length);
  for (let i = 0; i < a.length; i++) expect(Math.abs(a[i] - b[i])).toBeLessThan(eps);
};

// A two-bone arm like the game's: a top bone turned −90° about X (the "root"), then an elbow 1 m along its x.
const bones: Bone[] = [
  { name: 'root', path: 'root', parent: -1, pos: [0, 0, 0], rot: [-Math.SQRT1_2, 0, 0, Math.SQRT1_2], scale: [1, 1, 1] },
  { name: 'elbow', path: 'root/elbow', parent: 0, pos: [1, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
];

describe('skeleton', () => {
  it('inverts an affine matrix', () => {
    const m = trs([1, 2, 3], euler(10, 20, 30), [2, 2, 2]);
    close(mul4(m, invAffine(m)), [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
  });

  it('matches Unity Quaternion.Euler order (z, x, y)', () => {
    close(euler(0, 90, 0), [0, Math.SQRT1_2, 0, Math.SQRT1_2]);
    close(euler(30, 40, 50), qmul(euler(0, 40, 0), qmul(euler(30, 0, 0), euler(0, 0, 50))));
  });

  it('leaves the rest pose alone', () => {
    for (const m of skinMatrices(bones, { root: bones[0].rot })) close(m, [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
  });

  it('bends a vertex around its bone', () => {
    // Elbow turned 90° about its local z: a point 1 m past the elbow (x = 2 in the skeleton's space) swings round.
    const mats = skinMatrices(bones, { elbow: euler(0, 0, 90) });
    const out = skin(new Float32Array([2, 0, 0]), new Float32Array([1, 0, 0]), new Uint16Array([1, 0, 0, 0]), new Float32Array([1, 0, 0, 0]), mats);
    // root's −90° about X turns the elbow's local y into −z (Unity), +z in GL (z negated).
    close(out.pos, [1, 0, 1]);
    close(out.nrm, [0, 0, 1]);
  });
});

describe('animation helpers', () => {
  it('round-trips Unity euler angles', async () => {
    const { toEuler } = await import('./skeleton');
    close(toEuler(euler(20, -35, 50)), [20, -35, 50], 1e-3);
  });

  it('samples keys with easing, holding the ends', async () => {
    const { sampleKeys } = await import('./skeleton');
    const keys = [{ t: 0, r: [0, 0, 0, 1] }, { t: 1, r: euler(0, 0, 90) }];
    close(sampleKeys(keys, -1)!, [0, 0, 0, 1]);
    close(sampleKeys(keys, 2)!, euler(0, 0, 90));
    close(sampleKeys(keys, 0.5)!, euler(0, 0, 45), 1e-4); // smoothstep(0.5) = 0.5
  });

  it('turns a bone about a world axis through its parent', async () => {
    const { turnBone } = await import('./skeleton');
    // elbow's parent (root) is −90° about X, which turns local Z into world +Y: 90° about world Y = 90° about local Z.
    const q = turnBone(bones, {}, 1, [0, 1, 0], 90);
    close(q, euler(0, 0, 90), 1e-5);
  });

  it('masks bones by subtree', async () => {
    const { maskedBones } = await import('./skeleton');
    const b: Bone[] = [
      { name: 'root', path: 'root', parent: -1, pos: [0, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
      { name: 'spine_01', path: 'root/spine_01', parent: 0, pos: [0, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
      { name: 'clavicle_l', path: 'root/spine_01/clavicle_l', parent: 1, pos: [0, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
      { name: 'thigh_l', path: 'root/thigh_l', parent: 0, pos: [0, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
    ];
    expect([...maskedBones(b, 'upper')]).toEqual(['spine_01', 'clavicle_l']);
    expect([...maskedBones(b, 'arms')]).toEqual(['clavicle_l']);
    expect(maskedBones(b, 'full').size).toBe(4);
  });
});

describe('swinging a bone towards the mouse', () => {
  it('turns it the way the tip was moved, whichever way it points', async () => {
    const { swingBy, axisAngle, qmul, qconj } = (await import('./skeleton')) as any;
    const rotate = (q: number[], v: number[]) => {
      const r = qmul(qmul(q, [v[0], v[1], v[2], 0]), qconj(q));
      return [r[0], r[1], r[2]];
    };
    for (const dir of [[1, 0, 0], [-1, 0, 0], [0, 1, 0], [0, 0, 1], [0.6, 0.8, 0]]) {
      for (const move of [[0, 0.05, 0], [0, -0.05, 0], [0.05, 0, 0], [0, 0, 0.05]]) {
        const s = swingBy([0, 0, 0], dir, move, 0.3);
        if (!s) continue;
        const moved = rotate(axisAngle(s.axis, s.deg), dir);
        const went = moved.map((v: number, i: number) => v - dir[i]);
        expect(went[0] * move[0] + went[1] * move[1] + went[2] * move[2]).toBeGreaterThan(0);
      }
    }
  });

  it('turns short bones no faster than the lever allows', async () => {
    const { swingBy } = await import('./skeleton');
    const s = swingBy([0, 0, 0], [0.02, 0, 0], [0, 0.03, 0], 0.3)!;
    expect(s.deg).toBeLessThan(6);
  });
});

describe('natural posing', () => {
  // A straight arm along +x: shoulder at 0, elbow at 0.3, hand at 0.6 (each bone's local x points down the arm).
  const arm: Bone[] = [
    { name: 'upperarm_r', path: 'a', parent: -1, pos: [0, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
    { name: 'lowerarm_r', path: 'a/b', parent: 0, pos: [0.3, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
    { name: 'hand_r', path: 'a/b/c', parent: 1, pos: [0.3, 0, 0], rot: [0, 0, 0, 1], scale: [1, 1, 1] },
  ];
  it('reaches a target with two bones', async () => {
    const { twoBoneIK, worldMatrices } = await import('./skeleton');
    for (const target of [[0.3, 0.3, 0], [0.2, -0.1, 0.25], [0, 0.45, 0.1]]) {
      const p = twoBoneIK(arm, {}, 0, 1, 2, target, [0, 0, 1]);
      const w = worldMatrices(arm, p)[2];
      close([w[12], w[13], w[14]], target, 1e-3);
    }
  });
  it('reaches towards a target out of reach', async () => {
    const { twoBoneIK, worldMatrices } = await import('./skeleton');
    const p = twoBoneIK(arm, {}, 0, 1, 2, [0, 2, 0], [0, 0, 1]);
    const w = worldMatrices(arm, p)[2];
    close([w[12], w[13], w[14]], [0, 0.6, 0], 2e-3);
  });
  it('finds the chains', async () => {
    const { chainFor } = await import('./skeleton');
    expect(chainFor('hand_l')).toEqual({ kind: 'ik', upper: 'upperarm_l', lower: 'lowerarm_l', end: 'hand_l' });
    expect(chainFor('calf_r')).toMatchObject({ kind: 'ik', upper: 'thigh_r', end: 'foot_r' });
    expect(chainFor('index_02_r')).toEqual({ kind: 'curl', bones: ['index_02_r', 'index_03_r'] });
    expect(chainFor('spine_02')).toMatchObject({ kind: 'spread' });
    expect(chainFor('upperarm_l')).toBeNull();
  });
});

describe('animatableBones', () => {
  it('leaves out names used more than once, which the bone list and animations could not tell apart', () => {
    const bone = (name: string) => ({ name }) as Bone;
    const bones = [bone('hand_l'), bone('Ctrl'), bone('hand_r'), bone('Ctrl')];
    expect(bones.filter(animatableBones(bones)).map((b) => b.name)).toEqual(['hand_l', 'hand_r']);
  });
});
