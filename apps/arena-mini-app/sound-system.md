# Arena Mini-App Sound System

## Overview

The arena mini-app uses a centralized sound system built on React Context. Sounds are loaded from an external URL (MinIO/S3) and support preloading by category, audio pooling for overlapping sounds, and mobile autoplay unlock.

## Sound Files

All sounds are `.ogg` format stored at `{VITE_SOUND_BASE_URL}/{soundId}.ogg`.

### Sound Variants

Some sounds have multiple variants that are randomly selected at play time for variety. Variant sounds are stored in subdirectories:

| Sound ID | Variants | Path Pattern |
|----------|----------|--------------|
| `battle_attack` | 10 | `battle_attack/battle_attack_{1-10}.ogg` |
| `battle_damage` | 10 | `battle_damage/battle_damage_{1-10}.ogg` |

When `play('battle_attack')` is called, a random variant (1-10) is selected and played. All variants are preloaded together when the `battle` category is preloaded.

## Sound Registry

| Sound ID | Category | Description |
|----------|----------|-------------|
| `lobby_create` | lobby | Match created by user |
| `lobby_join` | lobby | User joined a match |
| `lobby_start` | lobby | Match transitioned to shop phase |
| `countdown_tick` | lobby | Timer tick (10s to 4s remaining) |
| `countdown_warning` | lobby | Timer warning (3s to 1s remaining) |
| `button_click` | shop/team | Generic button interaction |
| `card_draw` | shop | Cards dealt/drawn |
| `card_shuffle` | shop | Cards shuffled (reroll) |
| `coin_spend` | shop | Coins spent |
| `error` | shop/team | Action failed |
| `success` | shop | Action succeeded |
| `team_place` | team | Card placed/reordered in team |
| `team_upgrade` | team | Card upgraded |
| `battle_attack` | battle | Attacker initiates attack (10 variants) |
| `battle_damage` | battle | Defender takes damage (10 variants) |
| `battle_death` | battle | Card dies |
| `battle_win` | battle | User won the battle |
| `battle_lose` | battle | User lost the battle |
| `critical_hp` | battle | Card HP drops below 25% |

## Where Sounds Are Played

### LobbyPage.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `countdown_warning` | Timer at 3s, 2s, or 1s remaining | 87 |
| `countdown_tick` | Timer between 10s and 4s remaining | 92 |
| `lobby_start` | Match transitions to shop_phase (real-time) | 130 |
| `lobby_start` | Match details show shop_phase transition | 199 |
| `lobby_create` | User creates a new match | 279 |
| `lobby_join` | User joins an existing match | 301 |
| `lobby_start` | User manually starts match early | 351 |

### ShopPage.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `card_draw` | Initial shop cards loaded | 125 |
| `coin_spend` | Card purchased successfully | 196 |
| `success` | Card purchased (with coin_spend) | 197 |
| `error` | Card purchase failed | 201 |
| `card_shuffle` | Shop rerolled successfully | 237 |
| `card_draw` | Shop rerolled (new cards dealt) | 238 |
| `error` | Reroll failed | 242 |

### TeamPhaseModal.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `team_place` | Card drag-and-drop reorder completed | 170 |
| `error` | Reorder failed | 173 |
| `team_upgrade` | Card ATK/HP upgrade successful | 190 |
| `error` | Card upgrade failed | 193 |
| `button_click` | Team submitted successfully | 210 |
| `error` | Team submission failed | 213 |

### BattlePage.tsx (via useBattleAnimation hook)

| Sound | Trigger | Line (hook) |
|-------|---------|-------------|
| `battle_attack` | Attack animation starts (highlight phase) | 344 |
| `battle_damage` | Damage animation plays (attack phase) | 391 |
| `critical_hp` | Card HP drops below 25% threshold | 443 |
| `battle_death` | Card is_alive set to false | 531 |
| `battle_win` | Battle complete, user is winner | 202 (BattlePage) |
| `battle_lose` | Battle complete, user is loser | 202 (BattlePage) |

## Preloading Strategy

Sounds are preloaded by category to ensure they're ready when needed:

| Category | When Preloaded | Sounds |
|----------|----------------|--------|
| `lobby` | App mount (SoundProvider) | lobby_create, lobby_join, lobby_start, countdown_tick, countdown_warning |
| `shop` | ShopPage mount | button_click, card_draw, card_shuffle, coin_spend, error, success |
| `team` | ShopPage mount | team_place, team_upgrade, button_click, error |
| `battle` | BattlePage mount | battle_attack, battle_damage, battle_death, battle_win, battle_lose, critical_hp |

## Mobile Audio Unlock

Mobile browsers block autoplay until user interaction. The sound system:
1. Creates a silent audio element on mount
2. Plays silent audio on first user interaction (`unlockAudio()`)
3. This unlocks the audio context for subsequent sounds

`unlockAudio()` is called in LobbyPage via `ensureAudioUnlocked()` on match create/join/leave/start actions.

## Sequential Sound Playback

For actions that trigger multiple sounds, use `playSequence()` to play them in order with delays:

```tsx
const { playSequence } = useSoundContext()

// Simple usage with default 150ms delay between sounds
playSequence(['coin_spend', 'success'])

// Custom delay after specific sound (300ms after shuffle, then draw)
playSequence([['card_shuffle', 300], 'card_draw'])

// Custom default delay for all sounds
playSequence(['sound1', 'sound2', 'sound3'], { defaultDelay: 200 })
```

### Current Sequential Sound Pairs

| Location | Sounds | Purpose |
|----------|--------|---------|
| ShopPage - Card purchase | `coin_spend` → `success` | Purchase confirmation |
| ShopPage - Reroll | `card_shuffle` → `card_draw` | Shuffle then deal |
| ShopPage - Done shopping | `button_click` → `success` | Button + confirmation |
| TeamPhaseModal - Upgrade | `team_upgrade` | Upgrade sound |
