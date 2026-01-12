import { useState, useEffect, useCallback, useRef } from 'react';

type SoundName = 'attack' | 'hit' | 'ko' | 'victory' | 'place';

const SOUND_URLS: Record<SoundName, string> = {
  attack: '/sounds/attack.mp3',
  hit: '/sounds/hit.mp3',
  ko: '/sounds/ko.mp3',
  victory: '/sounds/victory.mp3',
  place: '/sounds/place.mp3',
};

const STORAGE_KEY = 'arena-audio-muted';

class AudioManager {
  private context: AudioContext | null = null;
  private buffers: Map<SoundName, AudioBuffer> = new Map();
  private gainNode: GainNode | null = null;
  private initialized = false;
  private loading = false;
  private muted = false;

  constructor() {
    // Check localStorage for muted preference
    const stored = localStorage.getItem(STORAGE_KEY);
    this.muted = stored === 'true';
  }

  private async initContext(): Promise<boolean> {
    if (this.context) return true;

    try {
      // Create AudioContext - handle webkit prefix for older Safari
      const AudioContextClass = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      if (!AudioContextClass) {
        console.warn('Web Audio API not supported');
        return false;
      }

      this.context = new AudioContextClass();
      this.gainNode = this.context.createGain();
      this.gainNode.connect(this.context.destination);
      this.gainNode.gain.value = this.muted ? 0 : 1;

      return true;
    } catch (err) {
      console.error('Failed to create AudioContext:', err);
      return false;
    }
  }

  async preload(): Promise<void> {
    if (this.initialized || this.loading) return;
    this.loading = true;

    const contextReady = await this.initContext();
    if (!contextReady || !this.context) {
      this.loading = false;
      return;
    }

    // Load all sounds in parallel
    const loadPromises = Object.entries(SOUND_URLS).map(async ([name, url]) => {
      try {
        const response = await fetch(url);
        if (!response.ok) {
          console.warn(`Failed to fetch sound: ${name}`);
          return;
        }
        const arrayBuffer = await response.arrayBuffer();
        const audioBuffer = await this.context!.decodeAudioData(arrayBuffer);
        this.buffers.set(name as SoundName, audioBuffer);
      } catch (err) {
        console.warn(`Failed to load sound ${name}:`, err);
      }
    });

    await Promise.all(loadPromises);
    this.initialized = true;
    this.loading = false;
  }

  async unlock(): Promise<void> {
    // Resume context if suspended (required for mobile)
    if (this.context?.state === 'suspended') {
      try {
        await this.context.resume();
      } catch (err) {
        console.warn('Failed to resume AudioContext:', err);
      }
    }
  }

  play(name: SoundName): void {
    if (this.muted || !this.context || !this.gainNode) return;

    const buffer = this.buffers.get(name);
    if (!buffer) return;

    // Ensure context is running
    if (this.context.state === 'suspended') {
      this.context.resume();
    }

    try {
      const source = this.context.createBufferSource();
      source.buffer = buffer;
      source.connect(this.gainNode);
      source.start(0);
    } catch (err) {
      console.warn(`Failed to play sound ${name}:`, err);
    }
  }

  setMuted(muted: boolean): void {
    this.muted = muted;
    localStorage.setItem(STORAGE_KEY, String(muted));

    if (this.gainNode) {
      this.gainNode.gain.value = muted ? 0 : 1;
    }
  }

  isMuted(): boolean {
    return this.muted;
  }
}

// Singleton instance
let audioManager: AudioManager | null = null;

function getAudioManager(): AudioManager {
  if (!audioManager) {
    audioManager = new AudioManager();
  }
  return audioManager;
}

export function useAudio() {
  const [muted, setMutedState] = useState(() => getAudioManager().isMuted());
  const preloadedRef = useRef(false);

  // Preload sounds on mount
  useEffect(() => {
    if (!preloadedRef.current) {
      preloadedRef.current = true;
      getAudioManager().preload();
    }
  }, []);

  // Handle user interaction to unlock audio on mobile
  useEffect(() => {
    const handleInteraction = () => {
      getAudioManager().unlock();
      // Remove listeners after first interaction
      document.removeEventListener('touchstart', handleInteraction);
      document.removeEventListener('click', handleInteraction);
    };

    document.addEventListener('touchstart', handleInteraction, { once: true });
    document.addEventListener('click', handleInteraction, { once: true });

    return () => {
      document.removeEventListener('touchstart', handleInteraction);
      document.removeEventListener('click', handleInteraction);
    };
  }, []);

  const play = useCallback((name: SoundName) => {
    getAudioManager().play(name);
  }, []);

  const setMuted = useCallback((value: boolean) => {
    getAudioManager().setMuted(value);
    setMutedState(value);
  }, []);

  const toggleMuted = useCallback(() => {
    const newValue = !getAudioManager().isMuted();
    getAudioManager().setMuted(newValue);
    setMutedState(newValue);
  }, []);

  return {
    play,
    muted,
    setMuted,
    toggleMuted,
  };
}

export type { SoundName };
