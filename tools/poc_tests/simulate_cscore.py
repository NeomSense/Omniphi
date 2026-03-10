#!/usr/bin/env python3
"""
Simple Monte Carlo simulator for C-Score dynamics.
Run: python tools/poc_tests/simulate_cscore.py --epochs 1000
Outputs CSV-summary: epoch, honest_avg, attacker_avg

Purpose: check for runaway growth, underflow/overflow, and effect of decay/caps.
"""
import argparse
import random
import csv


def simulate(epochs, honest_count, attacker_count, reward_per_epoch, decay_rate, cap):
    # initialize scores
    honest = [0.0 for _ in range(honest_count)]
    attacker = [0.0 for _ in range(attacker_count)]

    rows = []
    for e in range(epochs):
        # distribute rewards: honest do 'work' with p=0.9, attacker succeed with p=0.7
        for i in range(honest_count):
            if random.random() < 0.9:
                honest[i] += reward_per_epoch
        for i in range(attacker_count):
            if random.random() < 0.7:
                attacker[i] += reward_per_epoch * 1.2  # attacker may game rewards

        # apply decay and cap
        honest = [min(cap, s * (1 - decay_rate)) for s in honest]
        attacker = [min(cap, s * (1 - decay_rate)) for s in attacker]

        rows.append((e, sum(honest)/len(honest), sum(attacker)/len(attacker)))

    return rows


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--epochs', type=int, default=1000)
    p.add_argument('--honest', type=int, default=100)
    p.add_argument('--attacker', type=int, default=10)
    p.add_argument('--reward', type=float, default=1.0)
    p.add_argument('--decay', type=float, default=0.001)
    p.add_argument('--cap', type=float, default=1000.0)
    p.add_argument('--out', type=str, default='poc_cscore.csv')
    args = p.parse_args()

    rows = simulate(args.epochs, args.honest, args.attacker, args.reward, args.decay, args.cap)

    with open(args.out, 'w', newline='') as f:
        w = csv.writer(f)
        w.writerow(['epoch', 'honest_avg', 'attacker_avg'])
        for r in rows:
            w.writerow(r)

    print(f"Wrote results to {args.out}")
