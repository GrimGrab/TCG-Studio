# Makes the customers' built-in animations (studio/internal/customers/default-animations.json) from the game's skeleton (Studio's
# game-templates\characters\characters.json). A pose says where the arms point and how the torso and head turn, seen from the customer:
# +x their right, +y up, +z forward. Run it, optionally with a PNG path for a contact sheet of the poses:
#   python tools/make-default-animations.py [sheet.png]
import json
import math
import os
import struct
import sys
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, '..', 'studio', 'internal', 'customers', 'default-animations.json')
TEMPLATES = os.path.expandvars(r'%LOCALAPPDATA%\TCG Studio\game-templates\characters\characters.json')


def qmul(a, b):
    ax, ay, az, aw = a
    bx, by, bz, bw = b
    return (aw*bx+ax*bw+ay*bz-az*by, aw*by-ax*bz+ay*bw+az*bx, aw*bz+ax*by-ay*bx+az*bw, aw*bw-ax*bx-ay*by-az*bz)


def qconj(q):
    return (-q[0], -q[1], -q[2], q[3])


def rot(q, v):
    x, y, z, w = q
    c = (y*v[2]-z*v[1], z*v[0]-x*v[2], x*v[1]-y*v[0])
    d = (y*c[2]-z*c[1], z*c[0]-x*c[2], x*c[1]-y*c[0])
    return (v[0]+2*(w*c[0]+d[0]), v[1]+2*(w*c[1]+d[1]), v[2]+2*(w*c[2]+d[2]))


def axis(v, deg):
    s = math.sin(math.radians(deg)/2)
    return (v[0]*s, v[1]*s, v[2]*s, math.cos(math.radians(deg)/2))


def unit(v):
    n = math.sqrt(sum(a*a for a in v)) or 1
    return tuple(a/n for a in v)


def cross(a, b):
    return (a[1]*b[2]-a[2]*b[1], a[2]*b[0]-a[0]*b[2], a[0]*b[1]-a[1]*b[0])


def turn(lean=0, yaw=0, roll=0):
    """lean + = forward, yaw + = to their right, roll + = top towards their left."""
    return qmul(axis((0, 1, 0), yaw), qmul(axis((1, 0, 0), lean), axis((0, 0, 1), roll)))


def swing(a, b):
    """The smallest rotation taking direction a to b."""
    a, b = unit(a), unit(b)
    d = sum(x*y for x, y in zip(a, b))
    if d < -0.9999:
        return axis((0, 1, 0), 180)
    c = (a[1]*b[2]-a[2]*b[1], a[2]*b[0]-a[0]*b[2], a[0]*b[1]-a[1]*b[0])
    w = 1+d
    n = math.sqrt(c[0]**2+c[1]**2+c[2]**2+w*w)
    return (c[0]/n, c[1]/n, c[2]/n, w/n)


class Skeleton:
    def __init__(self, bones):
        self.bones = bones
        self.index = {}
        for i, b in enumerate(bones):
            self.index.setdefault(b['name'], i)
        self.world = []
        self.pos = []
        for b in bones:
            if b['parent'] < 0:
                self.world.append(tuple(b['rot']))
                self.pos.append(tuple(b['pos']))
            else:
                w = self.world[b['parent']]
                self.world.append(qmul(w, tuple(b['rot'])))
                p = rot(w, b['pos'])
                self.pos.append(tuple(self.pos[b['parent']][i]+p[i] for i in range(3)))

    def rest_world(self, n):
        return self.world[self.index[n]]

    def hand_axes(self, side, world):
        """Where a hand with this world rotation points its fingers and its palm."""
        to_world = qmul(world, qconj(self.rest_world('hand_'+side)))
        fingers = self.aim('hand_'+side, f'middle_01_{side}')
        i = self.index[f'middle_01_{side}']
        b = self.aim(f'middle_01_{side}', f'middle_02_{side}')
        curl = qmul(qmul(self.world[i], axis((0, 0, 1), -40)), qconj(self.world[i]))
        c = rot(curl, b)
        return rot(to_world, fingers), rot(to_world, tuple(c[k]-b[k] for k in range(3)))

    def aim(self, n, child):
        a, b = self.pos[self.index[n]], self.pos[self.index[child]]
        return unit(tuple(b[i]-a[i] for i in range(3)))


SPINE = ['spine_01', 'spine_02', 'spine_03']
ARMS_DOWN = ((0.2, -0.97, 0.05), (0.15, -0.9, 0.35))
FINGERS = ['index', 'middle', 'ring', 'pinky']
CLAP_UPPER = (0.08, -0.5, 0.86)
PALM_HALF = 0.016  # half a hand's thickness: wrists this far from the middle = palms touching
# These need hands at an exact place, so each body gets its own keys (arm lengths and shoulders differ).
PER_BODY = {'default_clap', 'default_pack_rip', 'default_card_look', 'default_thinking', 'default_hands_on_head', 'default_arms_crossed', 'default_facepalm', 'default_hands_on_hips'}


def reach(sk, upper, fore, x):
    """The forearm direction (only its sideways part changes) that puts the right wrist x metres from the middle."""
    length = lambda a, b: math.dist(sk.pos[sk.index[a]], sk.pos[sk.index[b]])
    at = lambda f: sk.pos[sk.index['upperarm_r']][0]+length('upperarm_r', 'lowerarm_r')*unit(upper)[0]+length('lowerarm_r', 'hand_r')*unit((f, fore[1], fore[2]))[0]
    lo, hi = -3.0, 3.0
    for _ in range(50):
        mid = (lo+hi)/2
        if at(mid) > x:
            hi = mid
        else:
            lo = mid
    return (lo, fore[1], fore[2])


def pose(sk, torso=(0, 0, 0), head=(0, 0, 0), r=ARMS_DOWN, l=ARMS_DOWN, shrug=0, fist_r=0, fist_l=0, thumb_r=False, foot_r=0, hands=None, hand_r=None, point_r=False, knee_r=0):
    """The pose as world rotations per bone. r / l = (upper arm, forearm) directions, each side as if it were the right one.
    hands = (fingers, palm) directions, the same way: both hands held like that, the palm being the side the fingers curl towards. hand_r = the same for the right hand alone.
    point_r = the right index finger stays out of the fist."""
    world = {}
    t = (0, 0, 0, 1)
    step = turn(*(a/3 for a in torso))
    for n in SPINE:
        t = qmul(step, t)
        world[n] = qmul(t, sk.rest_world(n))
    h = turn(*head)
    world['neck_01'] = qmul(qmul(turn(*(a*0.4 for a in head)), t), sk.rest_world('neck_01'))
    world['head'] = qmul(qmul(h, t), sk.rest_world('head'))
    for side, (upper, fore), mirror, curl in (('r', r, 1, fist_r), ('l', l, -1, fist_l)):
        m = lambda v: rot(t, (v[0]*mirror, v[1], v[2]))
        clav = turn(0, 0, -shrug*mirror)
        cw = qmul(qmul(clav, t), sk.rest_world('clavicle_'+side))
        world['clavicle_'+side] = cw
        base = qmul(qmul(clav, t), sk.rest_world('upperarm_'+side))
        now = rot(qmul(clav, t), sk.aim('upperarm_'+side, 'lowerarm_'+side))
        su = swing(now, m(upper))
        # An elbow is a hinge: turn the upper arm about itself so the forearm's bend is around the hinge, the way it is at rest.
        along = unit(m(upper))
        bend = cross(along, unit(m(fore)))
        if math.sqrt(sum(a*a for a in bend)) > 0.05:
            rest_bend = cross(sk.aim('upperarm_'+side, 'lowerarm_'+side), sk.aim('lowerarm_'+side, 'hand_'+side))
            rest_hinge = rot(sk.rest_world('lowerarm_'+side), (0, 0, 1))
            sign = 1 if sum(a*b for a, b in zip(rest_bend, rest_hinge)) > 0 else -1
            hinge = rot(qmul(su, qmul(clav, t)), rest_hinge)
            flat = lambda v: unit(tuple(v[i]-sum(a*b for a, b in zip(v, along))*along[i] for i in range(3)))
            a, b = flat(hinge), flat(tuple(sign*x for x in bend))
            d = max(-1, min(1, sum(x*y for x, y in zip(a, b))))
            side_of = sum(x*y for x, y in zip(cross(a, b), along))
            su = qmul(axis(along, math.degrees(math.acos(d))*(1 if side_of > 0 else -1)), su)
        world['upperarm_'+side] = qmul(su, base)
        carried = qmul(qmul(su, qmul(clav, t)), sk.rest_world('lowerarm_'+side))
        now = rot(qmul(su, qmul(clav, t)), sk.aim('lowerarm_'+side, 'hand_'+side))
        sf = swing(now, m(fore))
        world['lowerarm_'+side] = qmul(sf, carried)
        whole = qmul(sf, qmul(su, qmul(clav, t)))
        world['hand_'+side] = qmul(whole, sk.rest_world('hand_'+side))
        held = hands or (hand_r if side == 'r' else None)
        if held:
            fingers, palm = sk.hand_axes(side, world['hand_'+side])
            s = swing(fingers, m(held[0]))
            palm = rot(s, palm)
            want = unit(m(held[1]))
            along = unit(m(held[0]))
            # the palm turns about the fingers' line to where it should face
            flat = lambda v: unit(tuple(v[i]-sum(a*b for a, b in zip(v, along))*along[i] for i in range(3)))
            world['hand_'+side] = qmul(qmul(swing(flat(palm), flat(want)), s), world['hand_'+side])
        if curl:
            for f in FINGERS:
                if point_r and side == 'r' and f == 'index':
                    continue
                parent = world['hand_'+side]
                for k in ('01', '02', '03'):
                    n = f'{f}_{k}_{side}'
                    local = qmul(tuple(sk.bones[sk.index[n]]['rot']), axis((0, 0, 1), -curl))
                    parent = world[n] = qmul(parent, local)
            if side == 'r' and thumb_r:
                continue
            parent = world['hand_'+side]
            for k in ('01', '02', '03'):
                n = f'thumb_{k}_{side}'
                local = qmul(tuple(sk.bones[sk.index[n]]['rot']), axis((0, 0, 1), -curl*0.4))
                parent = world[n] = qmul(parent, local)
    if foot_r:
        world['foot_r'] = qmul(axis((1, 0, 0), -foot_r), sk.rest_world('foot_r'))
    if knee_r:
        # The knee comes up in front, the shin hangs a little behind it, the sole stays flat.
        world['thigh_r'] = qmul(axis((1, 0, 0), -knee_r), sk.rest_world('thigh_r'))
        world['calf_r'] = qmul(axis((1, 0, 0), -knee_r*0.25), sk.rest_world('calf_r'))
        world['foot_r'] = sk.rest_world('foot_r')
    return world


def skull(sk, torso, head):
    """Where the skull's centre is with the torso and head turned like this, and the head's forward and up directions."""
    world, pos = worlds(sk, locals_of(sk, pose(sk, torso=torso, head=head)))
    i = sk.index['head']
    up, forward = rot(world[i], (-1, 0, 0)), rot(world[i], (0, -1, 0))
    return tuple(pos[i][k]+up[k]*0.1 for k in range(3)), forward, up


def place(sk, kw, target):
    """The pose kw with its right forearm aimed so the wrist is at target."""
    kw = dict(kw)
    upper, fore = kw['r']
    t = turn(*kw.get('torso', (0, 0, 0)))
    length = math.dist(sk.pos[sk.index['lowerarm_r']], sk.pos[sk.index['hand_r']])
    reach_up = math.dist(sk.pos[sk.index['upperarm_r']], sk.pos[sk.index['lowerarm_r']])
    for _ in range(40):
        kw['r'] = (upper, fore)
        _, pos = worlds(sk, locals_of(sk, pose(sk, **kw)))
        at = pos[sk.index['hand_r']]
        err = rot(qconj(t), tuple(target[k]-at[k] for k in range(3)))
        # Mostly the forearm; the upper arm gives way when the forearm alone can't get there (arm lengths differ per body).
        fore = tuple(unit(fore)[k]+0.5*err[k]/length for k in range(3))
        upper = tuple(unit(upper)[k]+0.25*err[k]/reach_up for k in range(3))
    kw['r'] = (unit(upper), unit(fore))
    if math.dist(at, target) > 0.01:
        print('place: the wrist stays', round(math.dist(at, target)*100), 'cm from where it should be')
    return kw


def clips(sk):
    P = lambda **k: pose(sk, **k)
    rest = P()
    limp = ((0.08, -1, 0.15), (0.03, -1, 0.2))
    palm_out = lambda f: (f, (0, 0, 1))
    wave_a = dict(r=((0.8, 0.5, 0.15), (0.45, 0.9, 0.05)), head=(0, 6, 4), hand_r=palm_out((0.5, 0.86, 0.05)))
    wave_b = dict(r=((0.8, 0.5, 0.15), (-0.05, 1, 0.05)), head=(0, 6, 4), hand_r=palm_out((-0.15, 0.98, 0.05)))
    big_a = dict(r=((0.72, 0.68, 0.1), (0.42, 0.9, 0.05)), head=(-6, 8, 5), torso=(0, 0, 5), hand_r=palm_out((0.48, 0.87, 0.05)))
    big_b = dict(r=((0.72, 0.68, 0.1), (-0.05, 1, 0.05)), head=(-6, 8, 5), torso=(0, 0, -5), hand_r=palm_out((-0.12, 0.99, 0.05)))
    nod_d, nod_u = dict(head=(18, 0, 0)), dict(head=(-3, 0, 0))
    talk_upper = (0.35, -0.75, 0.55)
    talk_a = dict(r=(talk_upper, (0.25, 0.2, 0.95)), hand_r=((0.3, 0.2, 0.93), (0, 1, 0)), head=(3, 5, 0))
    talk_b = dict(r=(talk_upper, (0.5, 0.45, 0.74)), hand_r=((0.55, 0.4, 0.73), (0, 1, 0)), head=(-2, -5, 2))
    point = dict(r=((0.38, -0.22, 0.9), (0.08, 0.14, 0.99)), fist_r=80, point_r=True, torso=(5, 4, 0))
    point_b = dict(point, r=((0.38, -0.16, 0.91), (0.08, 0.24, 0.97)))
    wow_up = (0.7, 0.5, 0.12)
    wow_hands = ((-0.6, 0.78, 0.1), (-0.75, -0.6, 0.1))
    wow_skull = skull(sk, (-5, 0, 0), (-3, 0, 0))[0]
    wow = place(sk, dict(r=(wow_up, (-0.8, 0.3, 0.1)), hands=wow_hands, torso=(-5, 0, 0), head=(-3, 0, 0)), (wow_skull[0]+0.19, wow_skull[1]-0.07, wow_skull[2]+0.03))
    wow['l'] = wow['r']
    wow_b = dict(wow, head=(-3, 8, 0))
    wow_c = dict(wow, head=(-3, -8, 0))
    cross_upper = (0.18, -0.8, 0.57)
    crossed = dict(r=(cross_upper, reach(sk, cross_upper, (0, 0.32, 0.5), -0.08)), l=(cross_upper, reach(sk, cross_upper, (0, 0.15, 0.55), -0.08)), torso=(-3, 0, 0), head=(-3, 0, 0))
    crossed_b = dict(crossed, head=(-3, 15, 4))
    palm_upper = (0.1, -0.4, 0.9)
    face, ahead, above = skull(sk, (10, 0, 0), (30, 0, 0))
    palm_face = place(sk, dict(r=(palm_upper, (-0.5, 0.8, -0.2)), hand_r=((-0.2, 0.85, 0.45), (0, 0.5, -0.87)), torso=(10, 0, 0), head=(30, 0, 0)),
                      tuple(face[k]+ahead[k]*0.15-above[k]*0.1+(0.03 if k == 0 else 0) for k in range(3)))
    palm_face_b = dict(palm_face, head=(30, 9, 0))
    palm_face_c = dict(palm_face, head=(30, -9, 0))
    sneak_a = dict(head=(4, 42, 0), torso=(9, 8, 0), shrug=6)
    sneak_b = dict(head=(4, -42, 0), torso=(9, -8, 0), shrug=6)
    pelvis, thigh = sk.pos[sk.index['pelvis']], sk.pos[sk.index['thigh_r']]
    hips = place(sk, dict(r=((0.45, -0.82, -0.25), (-0.3, -0.7, 0.3)), hand_r=((-0.1, -0.6, 0.8), (-1, 0, 0)), torso=(-3, 0, 0), head=(0, 0, 8)),
                 (thigh[0]*1.9, pelvis[1]+0.12, pelvis[2]+0.02))
    hips['l'] = hips['r']
    hips['hands'] = hips.pop('hand_r')
    hips_b = dict(hips, head=(0, 0, -8))
    dfist_arm = (0.35, -0.85, 0.35)
    dfist_a = dict(r=(dfist_arm, (0, 0.5, 0.85)), l=(dfist_arm, (0, 0.5, 0.85)), fist_r=75, fist_l=75, torso=(-4, 0, 0), head=(-6, 0, 0))
    dfist_b = dict(r=(dfist_arm, (0.05, 0.05, 1)), l=(dfist_arm, (0.05, 0.05, 1)), fist_r=75, fist_l=75, torso=(8, 0, 0), head=(4, 0, 0))
    bowed = dict(torso=(32, 0, 0), head=(14, 0, 0), r=limp, l=limp)
    look_l_up, look_r_up = dict(head=(-9, -32, 0)), dict(head=(-9, 32, 0))
    pump_low = dict(r=((0.55, -0.55, 0.6), (0.05, 0.85, 0.5)), fist_r=75, torso=(4, 0, 0))
    pump_high = dict(r=((0.6, 0.15, 0.45), (0.1, 1, 0.1)), fist_r=75, torso=(-7, 0, 5), head=(-8, 0, 0))
    v_a = dict(r=((0.6, 0.8, 0.05), (0.35, 0.95, 0)), l=((0.6, 0.8, 0.05), (0.35, 0.95, 0)), fist_r=70, fist_l=70, torso=(-10, 0, 3), head=(-14, 0, 0), shrug=8)
    v_b = dict(r=((0.5, 0.85, 0.1), (0.15, 1, 0.05)), l=((0.5, 0.85, 0.1), (0.15, 1, 0.05)), fist_r=70, fist_l=70, torso=(-12, 0, -3), head=(-18, 0, 0), shrug=12)
    slump = dict(torso=(20, 0, 0), head=(24, 0, 0), r=limp, l=limp, shrug=-6)
    no_r, no_l = dict(head=(4, 24, 0)), dict(head=(4, -24, 0))
    tense = ((0.28, -0.96, 0.02), (0.2, -0.95, 0.24))
    stomp_up = dict(r=tense, l=tense, fist_r=80, fist_l=80, shrug=8, torso=(-4, 0, 0), head=(-4, 0, 0), knee_r=88)
    stomp_down = dict(r=tense, l=tense, fist_r=80, fist_l=80, shrug=3, torso=(14, 0, 0), head=(14, 0, 0))
    stomp_hold = dict(stomp_down, torso=(10, 0, 0), head=(8, 0, 0))
    thumb = dict(r=((0.35, -0.65, 0.65), (0.15, 0.3, 0.95)), fist_r=80, thumb_r=True)
    shrugging = dict(r=((0.5, -0.82, 0.15), (0.8, 0.1, 0.6)), l=((0.5, -0.82, 0.15), (0.8, 0.1, 0.6)), shrug=16, head=(0, 0, 10))
    clap_arm = lambda f: (CLAP_UPPER, f)
    clap_hands = ((0.12, 0.4, 0.9), (-1, 0, 0))
    clap_to = reach(sk, CLAP_UPPER, (0, 0.3, 0.92), PALM_HALF)
    clap_from = reach(sk, CLAP_UPPER, (0, 0.3, 0.92), 0.11)
    clap_open = dict(r=clap_arm(clap_from), l=clap_arm(clap_from), hands=clap_hands)
    clap_shut = dict(r=clap_arm(clap_to), l=clap_arm(clap_to), hands=clap_hands)
    # The left hand holds the pack at the chest, the right takes its top and tears it open; then both bring the cards up to look at.
    rip_lower, rip_upper = (0.1, -0.72, 0.68), (0.1, -0.55, 0.8)
    rip_left = reach(sk, rip_lower, (0, 0.3, 0.7), 0.03)
    rip_hold = dict(l=(rip_lower, rip_left), r=(rip_upper, reach(sk, rip_upper, (0, 0.55, 0.6), 0.03)), head=(24, 0, 0), torso=(6, 0, 0), fist_r=45, fist_l=45)
    rip_torn = dict(l=(rip_lower, rip_left), r=(rip_upper, reach(sk, rip_upper, (0, 0.7, 0.6), 0.13)), head=(22, 0, 0), torso=(5, 0, 0), fist_r=45, fist_l=45)
    look_r, look_l = (0.2, -0.55, 0.8), (0.15, -0.7, 0.7)
    look = dict(r=(look_r, reach(sk, look_r, (0, 0.75, 0.62), 0.06)), l=(look_l, reach(sk, look_l, (0, 0.45, 0.65), 0.1)), head=(14, 0, 0), torso=(3, 0, 0), fist_r=30, fist_l=30)
    look_b = dict(look, head=(14, 7, 3))
    think = dict(r=((0.1, -0.55, 0.83), (-0.58, 0.8, -0.2)), l=((0.25, -0.8, 0.55), (-0.75, 0.15, 0.62)), fist_r=55, head=(8, 0, 10))
    think_b = dict(think, head=(8, 12, 12))
    return [
        ('default_wave', 'Wave', 'upper', [(0, rest), (0.35, P(**wave_a)), (0.6, P(**wave_b)), (0.85, P(**wave_a)), (1.1, P(**wave_b)), (1.35, P(**wave_a)), (1.8, rest)]),
        ('default_fist_pump', 'Fist pump', 'upper', [(0, rest), (0.3, P(**pump_low)), (0.5, P(**pump_high)), (0.72, P(**pump_low)), (0.92, P(**pump_high)), (1.3, P(**pump_high)), (1.7, rest)]),
        ('default_celebrate', 'Big celebration', 'upper', [(0, rest), (0.35, P(**v_a)), (0.65, P(**v_b)), (0.95, P(**v_a)), (1.25, P(**v_b)), (1.55, P(**v_a)), (1.85, P(**v_b)), (2.4, rest)]),
        ('default_slump', 'Slump', 'upper', [(0, rest), (0.5, P(**slump)), (1.6, P(**slump)), (2.2, rest)]),
        ('default_head_shake', 'Head shake', 'upper', [(0, rest), (0.22, P(**no_r)), (0.5, P(**no_l)), (0.78, P(**no_r)), (1.06, P(**no_l)), (1.4, rest)]),
        ('default_stomp', 'Foot stomp', 'full', [(0, rest), (0.32, P(**stomp_up)), (0.44, P(**stomp_down)), (0.68, P(**stomp_hold)), (0.98, P(**stomp_up)), (1.1, P(**stomp_down)), (1.45, P(**stomp_hold)), (1.85, rest)]),
        ('default_thumbs_up', 'Thumbs up', 'upper', [(0, rest), (0.35, P(**thumb)), (0.55, P(**dict(thumb, head=(10, 0, 0)))), (0.8, P(**thumb)), (1.25, P(**thumb)), (1.6, rest)]),
        ('default_shrug', 'Shrug', 'upper', [(0, rest), (0.4, P(**shrugging)), (0.95, P(**shrugging)), (1.5, rest)]),
        ('default_foot_tap', 'Impatient foot tap', 'full', [(0, rest), (0.25, P(foot_r=18)), (0.5, P(foot_r=1)), (0.75, P(foot_r=18)), (1, P(foot_r=1)), (1.25, P(foot_r=18)), (1.5, P(foot_r=1)), (1.75, P(foot_r=18)), (2, rest)]),
        ('default_clap', 'Clap', 'upper', [(0, rest), (0.3, P(**clap_open)), (0.48, P(**clap_shut)), (0.66, P(**clap_open)), (0.84, P(**clap_shut)), (1.02, P(**clap_open)), (1.2, P(**clap_shut)), (1.65, rest)]),
        ('default_pack_rip', 'Pack rip', 'upper', [(0, rest), (0.4, P(**rip_hold)), (0.8, P(**rip_hold)), (0.98, P(**rip_torn)), (1.25, P(**rip_torn)), (1.6, rest)]),
        ('default_card_look', 'Looking at cards', 'upper', [(0, rest), (0.4, P(**look)), (1.3, P(**look_b)), (2.3, P(**look)), (3, P(**look)), (3.4, rest)]),
        ('default_thinking', 'Thinking', 'upper', [(0, rest), (0.5, P(**think)), (1.2, P(**think_b)), (1.9, P(**think)), (2.4, rest)]),
        ('default_big_wave', 'Big wave', 'upper', [(0, rest), (0.4, P(**big_a)), (0.7, P(**big_b)), (1, P(**big_a)), (1.3, P(**big_b)), (1.6, P(**big_a)), (2.1, rest)]),
        ('default_nod', 'Nod', 'upper', [(0, rest), (0.2, P(**nod_d)), (0.42, P(**nod_u)), (0.64, P(**nod_d)), (0.86, P(**nod_u)), (1.15, rest)]),
        ('default_talk', 'Talking hand', 'upper', [(0, rest), (0.35, P(**talk_a)), (0.75, P(**talk_b)), (1.15, P(**talk_a)), (1.55, P(**talk_b)), (2.05, rest)]),
        ('default_point', 'Point', 'upper', [(0, rest), (0.3, P(**point)), (0.55, P(**point_b)), (0.8, P(**point)), (1.3, P(**point)), (1.7, rest)]),
        ('default_hands_on_head', 'Hands on head', 'upper', [(0, rest), (0.4, P(**wow)), (0.85, P(**wow_b)), (1.3, P(**wow_c)), (1.75, P(**wow)), (2.2, rest)]),
        ('default_arms_crossed', 'Arms crossed', 'upper', [(0, rest), (0.45, P(**crossed)), (1.3, P(**crossed_b)), (2.1, P(**crossed)), (2.6, rest)]),
        ('default_facepalm', 'Facepalm', 'upper', [(0, rest), (0.4, P(**palm_face)), (0.75, P(**palm_face_b)), (1.1, P(**palm_face_c)), (1.45, P(**palm_face)), (1.95, rest)]),
        ('default_sneaky', 'Sneaky look', 'upper', [(0, rest), (0.4, P(**sneak_a)), (1, P(**sneak_a)), (1.45, P(**sneak_b)), (2, P(**sneak_b)), (2.4, rest)]),
        ('default_hands_on_hips', 'Hands on hips', 'upper', [(0, rest), (0.4, P(**hips)), (1.1, P(**hips_b)), (1.8, P(**hips)), (2.25, rest)]),
        ('default_double_fist', 'Double fist', 'upper', [(0, rest), (0.25, P(**dfist_a)), (0.45, P(**dfist_b)), (0.65, P(**dfist_a)), (0.85, P(**dfist_b)), (1.05, P(**dfist_a)), (1.5, rest)]),
        ('default_bow', 'Little bow', 'upper', [(0, rest), (0.45, P(**bowed)), (0.85, P(**bowed)), (1.35, rest)]),
        ('default_look_around', 'Look around', 'upper', [(0, rest), (0.5, P(**look_l_up)), (1.1, P(**look_l_up)), (1.7, P(**look_r_up)), (2.3, P(**look_r_up)), (2.8, rest)]),
    ]


MOVES = {
    'default_wave': ['empArrive', 'empGoHome', 'empGreet', 'greet', 'greetFan', 'friendHello', 'friendSpot', 'chatBye', 'leaveHappy'],
    'default_fist_pump': ['empBonus', 'ripHit', 'ripBigHit', 'winTable', 'beatPlayer', 'rivalWin', 'rivalBrick', 'cheerKick', 'cheerCatch'],
    'default_celebrate': ['empBonus', 'ripHugeHit', 'wishFound'],
    'default_slump': ['empFired', 'empFumble', 'ripBrick', 'ripBrickStack', 'loseTable', 'lostToPlayer', 'friendBrick', 'booKick'],
    'default_head_shake': ['empNoStock', 'boardShame', 'boardPulls', 'boardSpent', 'pricey', 'tradeRefuse', 'seeHustler', 'friendShamed'],
    'default_stomp': ['empCaughtThief', 'empFired', 'boardShameMe', 'greetUpset', 'kicked', 'caught', 'tradeRipped', 'rivalMeet', 'rivalSpot', 'challenge', 'rivalLose', 'rivalHit', 'gaveUp', 'tradeGaveUp', 'nickLost'],
    'default_thumbs_up': ['empRestockDone', 'boardBrag', 'boardTopMe', 'boardBoss', 'cheap', 'tip', 'tradeDone', 'friendMade', 'nearbyHit'],
    'default_shrug': ['empNoStock', 'empBreak', 'leaveEmpty', 'boardPacks', 'boardGossipMe'],
    'default_foot_tap': ['tooSlow', 'tradeTooSlow'],
    'default_clap': ['nickCoined', 'nickTaken', 'nearbyHit', 'nearbyHuge', 'friendHit', 'rivalShamed'],
    'default_pack_rip': ['ripPack'],
    'default_card_look': ['ripLook'],
    'default_thinking': ['empSetPrice', 'empIdle', 'boardRead', 'boardEmpty', 'boardHere', 'boardVisits', 'boardGossip', 'boardRivals', 'browseLook', 'wishAsk'],
    'default_big_wave': ['friendSpot', 'friendHello'],
    'default_nod': ['showOffReact', 'empGreet', 'empThanks', 'empRestockDone', 'upsellYes', 'chatReply', 'chatMoanReply', 'chatBragReply', 'chatDeckReply', 'chatPullReply', 'chatReactNice', 'chatSmallReply', 'chatAskReply', 'rivalAccept', 'nearbyHit'],
    'default_talk': ['empTillChat', 'showOffCard', 'sellOffer', 'empBounce', 'empMidday', 'ripChase', 'empChatMoan', 'empChatBrag', 'empChatDeck', 'empChatPull', 'empChatShop', 'empGossip', 'empUpsell', 'gossipNick', 'gossipNickBad', 'gossipNickTaken', 'chatOpen', 'chatOpenNews', 'chatMoan', 'chatBrag', 'chatAskDeck', 'chatAskPull', 'chatSmall', 'chatAsk', 'gossipTheft', 'gossipCaught', 'gossipKick', 'gossipTrade', 'gossipRivalry', 'gossipDuel', 'gossipFriends', 'gossipPull'],
    'default_point': ['empCaughtThief', 'empUpsell', 'nickCoinedBad', 'gawk', 'challenge', 'rivalSpot', 'tradeHustle', 'nearbyHit'],
    'default_hands_on_head': ['ripHugeHit', 'nearbyHuge', 'chatReact', 'gossipRipoff'],
    'default_arms_crossed': ['boardWins', 'boardRivalMine', 'tooSlow', 'tradeTooSlow', 'seeHustler', 'rivalMeet', 'greetUpset'],
    'default_facepalm': ['empFumble', 'ripBrick', 'ripBrickStack', 'loseTable', 'lostToPlayer'],
    'default_sneaky': ['sneak'],
    'default_hands_on_hips': ['tooSlow', 'pricey', 'tradeRefuse'],
    'default_double_fist': ['ripHit', 'winTable', 'beatPlayer', 'rivalWin', 'friendHit'],
    'default_bow': ['empThanks', 'tip', 'friendMade', 'chatBye'],
    'default_look_around': ['empIdle', 'empBreak', 'enter', 'browseLook'],
}


def locals_of(sk, world):
    """World rotations to the local ones a key stores (a bone without a world rotation of its own is at rest under its parent)."""
    out = {}
    def world_of(i):
        n = sk.bones[i]['name']
        if n in world and sk.index[n] == i:
            return world[n]
        return sk.world[i]
    for n, w in world.items():
        i = sk.index[n]
        out[n] = qmul(qconj(world_of(sk.bones[i]['parent'])), w)
    return out


def tracks(sk, keys):
    bones = {}
    posed = [(t, locals_of(sk, w)) for t, w in keys]
    for n in sorted({n for _, loc in posed for n in loc}):
        track = []
        for t, loc in posed:
            q = loc.get(n) or tuple(sk.bones[sk.index[n]]['rot'])
            track.append({'t': t, 'r': [round(v, 5) for v in unit(q)]})
        bones[n] = track
    return bones


def build(sk, female):
    anims = []
    other = {c[0]: c[3] for c in clips(female)}
    for id_, name, mask, keys in clips(sk):
        a = {'id': id_, 'name': name, 'length': keys[-1][0], 'mask': mask, 'bones': tracks(sk, keys)}
        if id_ in PER_BODY:
            a['femaleBones'] = tracks(female, other[id_])
        anims.append(a)
    return anims


# ---- contact sheet: the keys drawn back through the skeleton, seen from the front and the side

CELL = 150
LINKS = [('pelvis', 'spine_01'), ('spine_01', 'spine_02'), ('spine_02', 'spine_03'), ('spine_03', 'neck_01'), ('neck_01', 'head'),
         ('spine_03', 'clavicle_l'), ('clavicle_l', 'upperarm_l'), ('upperarm_l', 'lowerarm_l'), ('lowerarm_l', 'hand_l'),
         ('spine_03', 'clavicle_r'), ('clavicle_r', 'upperarm_r'), ('upperarm_r', 'lowerarm_r'), ('lowerarm_r', 'hand_r'),
         ('pelvis', 'thigh_l'), ('thigh_l', 'calf_l'), ('calf_l', 'foot_l'), ('pelvis', 'thigh_r'), ('thigh_r', 'calf_r'), ('calf_r', 'foot_r')]


def positions(sk, track_pose):
    world, pos = [], []
    for i, b in enumerate(sk.bones):
        q = track_pose.get(b['name']) if sk.index[b['name']] == i else None
        q = q or tuple(b['rot'])
        if b['parent'] < 0:
            world.append(q)
            pos.append(tuple(b['pos']))
        else:
            w = world[b['parent']]
            world.append(qmul(w, q))
            p = rot(w, b['pos'])
            pos.append(tuple(pos[b['parent']][k]+p[k] for k in range(3)))
    return pos


def sheet(sk, anims, path):
    frames = []
    for a in anims:
        times = sorted({k['t'] for keys in a['bones'].values() for k in keys})
        picks = times[1:4] if len(times) > 4 else times[1:-1]
        for t in picks[:3]:
            frames.append((a['name'], {n: tuple(next(k['r'] for k in keys if k['t'] == t)) for n, keys in a['bones'].items()}))
    cols = 6
    rows = math.ceil(len(frames)/3)
    W, H = cols*CELL, rows*CELL*2
    img = bytearray(b'\xff'*(W*H*3))

    def dot(x, y, c):
        if 0 <= x < W and 0 <= y < H:
            at = (int(y)*W+int(x))*3
            img[at:at+3] = bytes(c)

    def line(a, b, c):
        n = int(max(abs(a[0]-b[0]), abs(a[1]-b[1])))+1
        for i in range(n+1):
            x, y = a[0]+(b[0]-a[0])*i/n, a[1]+(b[1]-a[1])*i/n
            for dx in (0, 1):
                for dy in (0, 1):
                    dot(x+dx, y+dy, c)

    scale = CELL*2/2.3
    for f, (name, q) in enumerate(frames):
        pos = positions(sk, q)
        cx, cy = (f % 3)*2*CELL, (f//3)*2*CELL
        for view in (0, 1):
            ox = cx+view*CELL+CELL/2
            oy = cy+CELL*2-10
            for a, b in LINKS:
                p, r = pos[sk.index[a]], pos[sk.index[b]]
                c = (210, 40, 40) if a.endswith('_r') or b.endswith('_r') else (40, 80, 210) if a.endswith('_l') or b.endswith('_l') else (40, 40, 40)
                # front: seen from in front of them, their right on the left of the picture; side: they face right
                u = (lambda v: -v[0]) if view == 0 else (lambda v: v[2])
                line((ox+u(p)*scale, oy-p[1]*scale), (ox+u(r)*scale, oy-r[1]*scale), c)
        for y in range(H):
            if cx % (2*CELL) == 0:
                dot(cx, y, (220, 220, 220))
        for x in range(W):
            dot(x, cy, (220, 220, 220))

    def chunk(t, d):
        return struct.pack('>I', len(d))+t+d+struct.pack('>I', zlib.crc32(t+d))
    raw = b''.join(b'\0'+bytes(img[y*W*3:(y+1)*W*3]) for y in range(H))
    open(path, 'wb').write(b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 2, 0, 0, 0))+chunk(b'IDAT', zlib.compress(raw))+chunk(b'IEND', b''))
    print('sheet: rows of 3 poses, order', [a['name'] for a in anims])


def worlds(sk, q):
    world, pos = [], []
    for i, b in enumerate(sk.bones):
        r = (q.get(b['name']) if sk.index[b['name']] == i else None) or tuple(b['rot'])
        if b['parent'] < 0:
            world.append(r)
            pos.append(tuple(b['pos']))
        else:
            w = world[b['parent']]
            world.append(qmul(w, r))
            o = rot(w, b['pos'])
            pos.append(tuple(pos[b['parent']][k]+o[k] for k in range(3)))
    return world, pos


def to_segment(p, a, b):
    ab = tuple(b[i]-a[i] for i in range(3))
    u = max(0, min(1, sum((p[i]-a[i])*ab[i] for i in range(3))/(sum(x*x for x in ab) or 1)))
    return math.dist(p, tuple(a[i]+ab[i]*u for i in range(3)))


def critique(anims):
    """Measures every key on both bodies and prints what looks wrong: an elbow off its hinge, a twisted upper arm, a bent wrist, a hand
    or forearm inside the torso, a hand at the head that isn't, a bone moving too fast."""
    bodies = json.load(open(TEMPLATES, encoding='utf-8'))['bodies']
    problems = 0
    for a in anims:
        notes = {}
        for body in ('male', 'female'):
            sk = Skeleton(bodies[body]['skeleton'])
            tracks_ = a.get('femaleBones') if body == 'female' and a.get('femaleBones') else a['bones']
            times = sorted({k['t'] for keys in tracks_.values() for k in keys})
            prev = None
            first_q = {n: tuple(next(k['r'] for k in keys if k['t'] == times[0])) for n, keys in tracks_.items()}
            last_q = None
            for t in times:
                q = {n: tuple(next(k['r'] for k in keys if k['t'] == t)) for n, keys in tracks_.items()}
                if t == times[-1]:
                    last_q = q
                if t == times[0]:
                    q = first_q
                world, pos = worlds(sk, q)
                P = lambda n: pos[sk.index[n]]
                def note(kind, value):
                    notes[kind] = max(notes.get(kind, 0), value)
                for s in 'rl':
                    if f'lowerarm_{s}' in q:
                        rest = tuple(sk.bones[sk.index[f'lowerarm_{s}']]['rot'])
                        d = qmul(qconj(rest), q[f'lowerarm_{s}'])
                        note('elbow off hinge', math.degrees(2*math.asin(min(1, math.hypot(d[0], d[1])))))
                    if f'upperarm_{s}' in q:
                        rest = tuple(sk.bones[sk.index[f'upperarm_{s}']]['rot'])
                        d = qmul(qconj(rest), q[f'upperarm_{s}'])
                        note('upper arm twist', abs(math.degrees(2*math.atan2(d[0], d[3]))) if d[3] >= 0 else abs(math.degrees(2*math.atan2(-d[0], -d[3]))))
                    fa = unit(tuple(P(f'hand_{s}')[i]-P(f'lowerarm_{s}')[i] for i in range(3)))
                    hd = unit(tuple(P(f'middle_01_{s}')[i]-P(f'hand_{s}')[i] for i in range(3)))
                    note('wrist bend', math.degrees(math.acos(max(-1, min(1, sum(x*y for x, y in zip(fa, hd)))))))
                    spine = ['pelvis', 'spine_01', 'spine_02', 'spine_03', 'neck_01']
                    mid = tuple((P(f'hand_{s}')[i]+P(f'lowerarm_{s}')[i])/2 for i in range(3))
                    for label, pt in (('hand', P(f'hand_{s}')), ('forearm', mid)):
                        near = min(to_segment(pt, P(x), P(y)) for x, y in zip(spine, spine[1:]))
                        note(f'{label} too close to torso (cm under 11)', max(0, (0.11-near)*100))
                if prev:
                    pt, pq = prev
                    for n, r in q.items():
                        dot = abs(sum(x*y for x, y in zip(r, pq[n])))
                        note('fastest bone deg/s', math.degrees(2*math.acos(min(1, dot)))/(t-pt))
                        if n.startswith('upperarm') and pq is not first_q and q is not last_q:
                            note('upper arm spin between poses', math.degrees(2*math.acos(min(1, dot))))
                prev = (t, q)
                for s in 'rl':
                    note(f'hand height {s} (cm above head)', (P(f'hand_{s}')[1]-P('head')[1])*100 + 500)
        limits = {'upper arm spin between poses': 60, 'elbow off hinge': 15, 'upper arm twist': 95, 'wrist bend': 65, 'hand too close to torso (cm under 11)': 0.5, 'forearm too close to torso (cm under 11)': 0.5, 'fastest bone deg/s': 800}
        bad = {k: round(v) for k, v in notes.items() if k in limits and v > limits[k]}
        problems += len(bad)
        print(f"{a['id'][8:]:16} {bad if bad else 'ok'}  (hand peak r {round(notes['hand height r (cm above head)']-500)} l {round(notes['hand height l (cm above head)']-500)} cm from head)")
    print('critique:', problems, 'flag(s)')


def check_head(anims):
    """Hands that touch the head: every hand point's distance from the skull's centre, in cm. Under about 9 = inside the head."""
    bodies = json.load(open(TEMPLATES, encoding='utf-8'))['bodies']
    points = ['hand_r', 'middle_01_r', 'middle_03_r', 'index_03_r', 'pinky_03_r', 'thumb_03_r']
    a = next(x for x in anims if x['id'] == 'default_hands_on_hips')
    for body in ('male', 'female'):
        sk = Skeleton(bodies[body]['skeleton'])
        q = {n: tuple(next(k['r'] for k in keys if k['t'] == 0.4)) for n, keys in a['femaleBones' if body == 'female' else 'bones'].items()}
        _, pos = worlds(sk, q)
        print('hips', body, 'pelvis', [round(v, 3) for v in pos[sk.index['pelvis']]], {n[:-2]: [round(v, 3) for v in pos[sk.index[n]]] for n in ('hand_r', 'middle_03_r', 'lowerarm_r')})
    for id_ in ('default_hands_on_head', 'default_facepalm'):
        a = next(x for x in anims if x['id'] == id_)
        for body in ('male', 'female'):
            sk = Skeleton(bodies[body]['skeleton'])
            q = {n: tuple(next(k['r'] for k in keys if k['t'] == 0.4)) for n, keys in a['femaleBones' if body == 'female' else 'bones'].items()}
            world, pos = worlds(sk, q)
            i = sk.index['head']
            c = tuple(pos[i][k]+rot(world[i], (-0.1, 0, 0))[k] for k in range(3))
            print(id_[8:], body, {n[:-2]: round(math.dist(pos[sk.index[n]], c)*100) for n in points})


def check_clap(anims):
    """How far the palms' centres are from the middle at the clap's shut and open poses, on each body (metres; negative = they cross)."""
    clap = next(a for a in anims if a['id'] == 'default_clap')
    templates = json.load(open(TEMPLATES, encoding='utf-8'))['bodies']
    for t in (0.48, 0.66):
        for body in ('male', 'female'):
            q = {n: tuple(next(k['r'] for k in keys if k['t'] == t)) for n, keys in clap['femaleBones' if body == 'female' else 'bones'].items()}
            sk = Skeleton(templates[body]['skeleton'])
            pos = positions(sk, q)
            print(f'clap at {t} {body}:', {n: [round(c, 3) for c in pos[sk.index[n]]] for n in ('hand_r', 'middle_01_r', 'middle_03_r', 'hand_l', 'middle_01_l')})


def main():
    bodies = json.load(open(TEMPLATES, encoding='utf-8'))['bodies']
    sk = Skeleton(bodies['male']['skeleton'])
    anims = build(sk, Skeleton(bodies['female']['skeleton']))
    moves = {}
    for id_, moments in MOVES.items():
        for m in moments:
            moves.setdefault(m, []).append(id_)
    with open(OUT, 'w', encoding='utf-8', newline='\n') as f:
        json.dump({'animations': anims, 'moves': moves}, f, separators=(',', ':'))
    print(len(anims), 'animations,', len(moves), 'moments,', os.path.getsize(OUT), 'bytes')
    if len(sys.argv) > 1:
        sheet(sk, anims[int(sys.argv[2]) if len(sys.argv) > 2 else 0:], sys.argv[1])
    check_clap(anims)
    critique(anims)
    check_head(anims)


main()
