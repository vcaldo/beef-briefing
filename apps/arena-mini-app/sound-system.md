# Arena Mini-App Sound System

## Overview

The arena mini-app uses a centralized sound system built on React Context. Sounds are loaded from an external URL (MinIO/S3) and support preloading by category, audio pooling for overlapping sounds, and mobile autoplay unlock.

## Sound Files

All sounds are `.ogg` format stored at `{VITE_SOUND_BASE_URL}/{soundId}.ogg`.

## Sound Registry

| Sound ID | Category | Description |
|----------|----------|-------------|
| `lobby_create` | lobby | Match created by user |
| `lobby_join` | lobby | User joined a match |
| `lobby_start` | lobby | Match transitioned to shop phase |
| `countdown_tick` | lobby | Timer tick (10s to 4s remaining) |
| `countdown_warning` | lobby | Timer warning (3s to 1s remaining) |
| `tab_switch` | lobby | User switched navigation tabs |
| `shop_reroll` | shop | Shop cards rerolled |
| `shop_buy` | shop | Card purchased from shop |
| `button_click` | shop/team | Generic button interaction |
| `card_draw` | shop | Cards dealt/drawn |
| `card_shuffle` | shop | Cards shuffled (reroll) |
| `coin_spend` | shop | Coins spent |
| `error` | shop/team | Action failed |
| `card_hover` | shop | Card hovered (unused) |
| `success` | shop | Action succeeded |
| `team_place` | team | Card placed/reordered in team |
| `team_upgrade` | team | Card upgraded |
| `modal_open` | team | Modal opened |
| `modal_close` | team | Modal closed |
| `team_ready` | team | Team submitted |
| `powerup` | team | Upgrade effect |
| `battle_attack` | battle | Attacker initiates attack |
| `battle_damage` | battle | Defender takes damage |
| `battle_death` | battle | Card dies |
| `battle_win` | battle | User won the battle |
| `battle_lose` | battle | User lost the battle |
| `round_start` | battle | Battle round begins (unused) |
| `critical_hp` | battle | Card HP drops below 25% |

## Where Sounds Are Played

### TabBar.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `tab_switch` | User clicks a different tab | 126 |

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
| `shop_buy` | Card purchased successfully | 196 |
| `coin_spend` | Card purchased (with shop_buy) | 197 |
| `success` | Card purchased (with shop_buy) | 198 |
| `error` | Card purchase failed | 202 |
| `card_shuffle` | Shop rerolled successfully | 237 |
| `card_draw` | Shop rerolled (new cards dealt) | 238 |
| `error` | Reroll failed | 242 |

### TeamPhaseModal.tsx

| Sound | Trigger | Line |
|-------|---------|------|
| `modal_open` | Modal mounts | 222 |
| `modal_close` | Modal unmounts | 224 |
| `team_place` | Card drag-and-drop reorder completed | 170 |
| `error` | Reorder failed | 173 |
| `team_upgrade` | Card ATK/HP upgrade successful | 190 |
| `powerup` | Card upgrade (with team_upgrade) | 191 |
| `error` | Card upgrade failed | 194 |
| `button_click` | Team submitted successfully | 210 |
| `team_ready` | Team submitted (with button_click) | 211 |
| `error` | Team submission failed | 214 |

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
| `lobby` | App mount (SoundProvider) | lobby_create, lobby_join, lobby_start, countdown_tick, countdown_warning, tab_switch |
| `shop` | ShopPage mount | shop_reroll, shop_buy, button_click, card_draw, card_shuffle, coin_spend, error, card_hover, success |
| `team` | ShopPage mount | team_place, team_upgrade, button_click, modal_open, modal_close, team_ready, powerup, error |
| `battle` | BattlePage mount | battle_attack, battle_damage, battle_death, battle_win, battle_lose, round_start, critical_hp |

## Mobile Audio Unlock

Mobile browsers block autoplay until user interaction. The sound system:
1. Creates a silent audio element on mount
2. Plays silent audio on first user interaction (`unlockAudio()`)
3. This unlocks the audio context for subsequent sounds

`unlockAudio()` is called in LobbyPage via `ensureAudioUnlocked()` on match create/join/leave/start actions.

## Unused Sounds

The following sounds are defined but not currently triggered:
- `shop_reroll` - Defined but `card_shuffle` is used instead for reroll
- `card_hover` - Defined but no hover sound implementation
- `round_start` - Defined but not used in battle animation
