# Arena Mini-App Sound System

## Overview

The arena mini-app uses a centralized sound system built on React Context. Sounds are loaded from an external URL (MinIO/S3) and support preloading by category, audio pooling for overlapping sounds, and mobile autoplay unlock.

## Sound Files

All sounds are `.ogg` format stored at `{VITE_SOUND_BASE_URL}/{soundId}.ogg`.

### Sound Variants

Some sounds have multiple variants that are randomly selected at play time for variety. Variant sounds use a numbered suffix:

| Sound ID | Variants | Path Pattern |
|----------|----------|--------------|
| `arena_battle_attack` | 10 | `arena_battle_attack_{1-10}.ogg` |
| `arena_battle_damage` | 10 | `arena_battle_damage_{1-10}.ogg` |

When `play('arena_battle_attack')` is called, a random variant (1-10) is selected and played. All variants are preloaded together when the `battle` category is preloaded.

## Sound Registry

| Sound ID | Category | Description |
|----------|----------|-------------|
| `arena_lobby_create` | lobby | Match created by user |
| `arena_lobby_join` | lobby | User joined a match |
| `arena_countdown_tick` | lobby | Timer tick (10s to 4s remaining) |
| `arena_countdown_warning` | lobby | Timer warning (3s to 1s remaining) |
| `arena_button_click` | shop/team | Generic button interaction |
| `arena_card_draw` | shop | Cards dealt/drawn |
| `arena_card_shuffle` | shop | Cards shuffled (reroll) |
| `arena_coin_spend` | shop | Coins spent |
| `arena_error` | shop/team | Action failed |
| `arena_success` | shop | Action succeeded |
| `arena_team_place` | team | Card placed/reordered in team |
| `arena_team_upgrade` | team | Card upgraded |
| `arena_battle_attack` | battle | Attacker initiates attack (10 variants) |
| `arena_battle_damage` | battle | Defender takes damage (10 variants) |
| `arena_battle_death` | battle | Card dies |
| `arena_battle_win` | battle | User won the battle |
| `arena_battle_lose` | battle | User lost the battle |
| `arena_critical_hp` | battle | Card HP drops below 25% |

## Where Sounds Are Played

### LobbyPage.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `arena_countdown_warning` | Timer at 3s, 2s, or 1s remaining | 87 |
| `arena_countdown_tick` | Timer between 10s and 4s remaining | 92 |
| `arena_lobby_create` | User creates a new match | 278 |
| `arena_lobby_join` | User joins an existing match | 300 |

### ShopPage.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `arena_card_draw` | Initial shop cards loaded | 125 |
| `arena_coin_spend` | Card purchased successfully | 196 |
| `arena_success` | Card purchased (with coin_spend) | 197 |
| `arena_error` | Card purchase failed | 201 |
| `arena_card_shuffle` | Shop rerolled successfully | 237 |
| `arena_card_draw` | Shop rerolled (new cards dealt) | 238 |
| `arena_error` | Reroll failed | 242 |

### TeamPhaseModal.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `arena_team_place` | Card drag-and-drop reorder completed | 170 |
| `arena_error` | Reorder failed | 173 |
| `arena_team_upgrade` | Card ATK/HP upgrade successful | 190 |
| `arena_error` | Card upgrade failed | 193 |
| `arena_button_click` | Team submitted successfully | 210 |
| `arena_error` | Team submission failed | 213 |

### BattlePage.tsx (via useBattleAnimation hook)

| Sound | Trigger | Line (hook) |
|-------|---------|-------------|
| `arena_battle_attack` | Attack animation starts (highlight phase) | 344 |
| `arena_battle_damage` | Damage animation plays (attack phase) | 391 |
| `arena_critical_hp` | Card HP drops below 25% threshold | 443 |
| `arena_battle_death` | Card is_alive set to false | 531 |
| `arena_battle_win` | Battle complete, user is winner | 202 (BattlePage) |
| `arena_battle_lose` | Battle complete, user is loser | 202 (BattlePage) |

## Preloading Strategy

Sounds are preloaded by category to ensure they're ready when needed:

| Category | When Preloaded | Sounds |
|----------|----------------|--------|
| `lobby` | App mount (SoundProvider) | arena_lobby_create, arena_lobby_join, arena_countdown_tick, arena_countdown_warning |
| `shop` | ShopPage mount | arena_button_click, arena_card_draw, arena_card_shuffle, arena_coin_spend, arena_error, arena_success |
| `team` | ShopPage mount | arena_team_place, arena_team_upgrade, arena_button_click, arena_error |
| `battle` | BattlePage mount | arena_battle_attack, arena_battle_damage, arena_battle_death, arena_battle_win, arena_battle_lose, arena_critical_hp |

## Mobile Audio Unlock

Mobile browsers block autoplay until user interaction. The sound system:
1. Creates a silent audio element on mount
2. Plays silent audio on first user interaction (`unlockAudio()`)
3. This unlocks the audio context for subsequent sounds

`unlockAudio()` is called in LobbyPage via `ensureAudioUnlocked()` on match create/join/leave/start actions.

## Audio Context Wake-Up (Primer System)

After extended periods of inactivity, browser audio contexts can go dormant, causing the first sound after idle to fail. To prevent this:

1. A near-silent "primer" sound (`arena_primer.ogg`) is preloaded on mount
2. Before **every** sound plays, the primer is played first to wake up the audio context
3. The primer is ~50ms of barely-audible white noise (volume 0.01)
4. This ensures audio playback is reliable even after long idle periods

The primer is internal to the sound system and not exposed as a public `SoundId`.

## Sequential Sound Playback

For actions that trigger multiple sounds, use `playSequence()` to play them in order with delays:

```tsx
const { playSequence } = useSoundContext()

// Simple usage with default 150ms delay between sounds
playSequence(['arena_coin_spend', 'arena_success'])

// Custom delay after specific sound (300ms after shuffle, then draw)
playSequence([['arena_card_shuffle', 300], 'arena_card_draw'])

// Custom default delay for all sounds
playSequence(['sound1', 'sound2', 'sound3'], { defaultDelay: 200 })
```

### Current Sequential Sound Pairs

| Location | Sounds | Purpose |
|----------|--------|---------|
| ShopPage - Card purchase | `arena_coin_spend` → `arena_success` | Purchase confirmation |
| ShopPage - Reroll | `arena_card_shuffle` → `arena_card_draw` | Shuffle then deal |
| ShopPage - Done shopping | `arena_button_click` → `arena_success` | Button + confirmation |
| TeamPhaseModal - Upgrade | `arena_team_upgrade` | Upgrade sound |
