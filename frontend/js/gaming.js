/**
 * Guardians of the Lake - Gamification & Mini-Game Engine
 * File: frontend/js/gaming.js
 * 
 * Features:
 * - Guardian XP, Level Progression & Lightning Sats Wallet HUD
 * - Daily & Weekly Limnological Eco-Quests with instant reward claims
 * - Guardian Hall of Fame (Live Leaderboard integration)
 * - Achievement Badges Showcase
 * - Arcade Mini-Game: "Lake Victoria Clean-Up Blitz" (Canvas radar simulator)
 * - Web Audio API synthesizer for retro sound effects
 */

import { api } from './api.js';

// Default initial state
const DEFAULT_GAME_STATE = {
  xp: 1450,
  level: 7,
  title: 'Senior Lake Sentinel',
  satsBalance: 620,
  streakDays: 7,
  multiplier: 2.0,
  quests: {
    patrol: { id: 'patrol', name: 'Beach Patrol', desc: 'Verify 2 community pollution reports', current: 2, target: 2, xp: 150, sats: 25, claimed: false },
    turbidity: { id: 'turbidity', name: 'Water Clarity Scout', desc: 'Submit a photo of lake turbidity', current: 1, target: 1, xp: 200, sats: 50, claimed: false },
    ai_sentry: { id: 'ai_sentry', name: 'AI Sentry Validator', desc: 'Validate an automated AI anomaly prediction', current: 1, target: 1, xp: 120, sats: 30, claimed: false },
    streak: { id: 'streak', name: '7-Day Guardian Streak', desc: 'Maintain active surveillance 7 days in a row', current: 7, target: 7, xp: 300, sats: 100, claimed: false },
    blitz: { id: 'blitz', name: 'Clean-Up Blitz Hero', desc: 'Score over 400 pts in the Clean-Up Blitz mini-game', current: 0, target: 400, xp: 250, sats: 75, claimed: false }
  },
  stats: {
    reportsCount: 14,
    verificationsCount: 38,
    litresProtected: 2850,
    blitzHighscore: 520,
    accuracy: 98.4
  }
};

class GameEngine {
  constructor() {
    this.state = this.loadState();
    this.audioCtx = null;
    this.gameLoopId = null;
    this.isGameRunning = false;
    this.gameScore = 0;
    this.gameTimeLeft = 30;
    this.gameCombo = 1.0;
    this.gameTargets = [];
    this.gameParticles = [];
  }

  loadState() {
    try {
      const saved = localStorage.getItem('lake_guardians_game_state');
      if (saved) {
        const parsed = JSON.parse(saved);
        return { ...DEFAULT_GAME_STATE, ...parsed, quests: { ...DEFAULT_GAME_STATE.quests, ...(parsed.quests || {}) } };
      }
    } catch (e) {
      console.warn('[Gaming] Error loading game state:', e);
    }
    return JSON.parse(JSON.stringify(DEFAULT_GAME_STATE));
  }

  saveState() {
    try {
      localStorage.setItem('lake_guardians_game_state', JSON.stringify(this.state));
    } catch (e) {}
    this.updateHUD();
  }

  init() {
    this.updateHUD();
    this.renderQuestsScreen();
    this.bindEvents();
  }

  bindEvents() {
    // Mini-Game Trigger Buttons
    const openGameBtns = document.querySelectorAll('.launch-blitz-btn');
    openGameBtns.forEach(btn => {
      btn.addEventListener('click', () => this.openGameModal());
    });

    const closeGameBtn = document.getElementById('close-blitz-modal');
    if (closeGameBtn) {
      closeGameBtn.addEventListener('click', () => this.closeGameModal());
    }

    const startBtn = document.getElementById('start-blitz-btn');
    if (startBtn) {
      startBtn.addEventListener('click', () => this.startGame());
    }

    const restartBtn = document.getElementById('restart-blitz-btn');
    if (restartBtn) {
      restartBtn.addEventListener('click', () => this.startGame());
    }

    const claimPayoutBtn = document.getElementById('claim-blitz-payout-btn');
    if (claimPayoutBtn) {
      claimPayoutBtn.addEventListener('click', () => this.claimMiniGamePayout());
    }
  }

  // =========================================================================
  // Web Audio Synthesizer (Pure JS 8-bit sound effects)
  // =========================================================================

  getAudioContext() {
    if (!this.audioCtx) {
      const AudioContext = window.AudioContext || window.webkitAudioContext;
      if (AudioContext) this.audioCtx = new AudioContext();
    }
    if (this.audioCtx && this.audioCtx.state === 'suspended') {
      this.audioCtx.resume();
    }
    return this.audioCtx;
  }

  playBeep(freq = 440, duration = 0.08, type = 'sine') {
    try {
      const ctx = this.getAudioContext();
      if (!ctx) return;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = type;
      osc.frequency.setValueAtTime(freq, ctx.currentTime);
      gain.gain.setValueAtTime(0.15, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + duration);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + duration);
    } catch (e) {}
  }

  playZap() {
    try {
      const ctx = this.getAudioContext();
      if (!ctx) return;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sawtooth';
      osc.frequency.setValueAtTime(880, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(110, ctx.currentTime + 0.2);
      gain.gain.setValueAtTime(0.2, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.2);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.2);
    } catch (e) {}
  }

  playLevelUp() {
    try {
      const ctx = this.getAudioContext();
      if (!ctx) return;
      const notes = [523.25, 659.25, 783.99, 1046.50]; // C5, E5, G5, C6
      notes.forEach((freq, i) => {
        setTimeout(() => this.playBeep(freq, 0.12, 'triangle'), i * 80);
      });
    } catch (e) {}
  }

  playSuccess() {
    try {
      const ctx = this.getAudioContext();
      if (!ctx) return;
      this.playBeep(587.33, 0.09, 'sine'); // D5
      setTimeout(() => this.playBeep(880, 0.15, 'sine'), 90); // A5
    } catch (e) {}
  }

  // =========================================================================
  // Progression & Reward Actions
  // =========================================================================

  addXP(amount, reason = '') {
    this.state.xp += amount;
    const oldLevel = this.state.level;
    const nextLevelXP = this.getXPForLevel(oldLevel + 1);
    
    if (this.state.xp >= nextLevelXP) {
      this.state.level += 1;
      this.state.title = this.getTitleForLevel(this.state.level);
      this.playLevelUp();
      this.showToast(`🎉 LEVEL UP! You reached Level ${this.state.level}: ${this.state.title}!`, 'success');
      this.triggerConfetti();
    } else {
      this.playBeep(660, 0.1);
      if (reason) {
        this.showToast(`⭐ +${amount} XP (${reason})`, 'info');
      }
    }
    this.saveState();
  }

  addSats(amount, reason = '') {
    this.state.satsBalance += amount;
    this.playZap();
    if (reason) {
      this.showToast(`⚡ +${amount} SATS paid to LNbits wallet (${reason})`, 'lightning');
    }
    this.saveState();
  }

  getXPForLevel(lvl) {
    return Math.round(100 * Math.pow(lvl, 1.4));
  }

  getTitleForLevel(lvl) {
    if (lvl >= 12) return 'Limnology Vanguard';
    if (lvl >= 9) return 'Master Lake Guardian';
    if (lvl >= 6) return 'Senior Lake Sentinel';
    if (lvl >= 4) return 'Trusted Water Scout';
    if (lvl >= 2) return 'Active Beach Sentinel';
    return 'Junior Observer';
  }

  awardReportSubmission() {
    this.addXP(200, 'New Incident Report');
    this.addSats(50, 'Report Bounty');
    this.state.stats.reportsCount += 1;
    this.state.quests.turbidity.current = Math.min(this.state.quests.turbidity.target, this.state.quests.turbidity.current + 1);
    this.saveState();
    this.renderQuestsScreen();
  }

  awardVerificationVote() {
    this.addXP(60, 'Consensus Peer Review');
    this.addSats(15, 'Verifier Reward');
    this.state.stats.verificationsCount += 1;
    this.state.quests.patrol.current = Math.min(this.state.quests.patrol.target, this.state.quests.patrol.current + 1);
    this.saveState();
    this.renderQuestsScreen();
  }

  claimQuest(questId) {
    const q = this.state.quests[questId];
    if (!q || q.claimed || q.current < q.target) return;

    q.claimed = true;
    this.addXP(q.xp, `${q.name} Quest Complete`);
    this.addSats(q.sats, `${q.name} Quest Reward`);
    this.triggerConfetti();
    this.saveState();
    this.renderQuestsScreen();
  }

  // =========================================================================
  // HUD Rendering
  // =========================================================================

  updateHUD() {
    // Level & XP
    const lvlEl = document.getElementById('hud-user-level');
    const titleEl = document.getElementById('hud-user-title');
    const xpBarEl = document.getElementById('hud-xp-bar');
    const xpTextEl = document.getElementById('hud-xp-text');
    const satsEl = document.getElementById('hud-sats-balance');
    const streakEl = document.getElementById('hud-streak-count');

    const currentLvlXP = this.getXPForLevel(this.state.level);
    const nextLvlXP = this.getXPForLevel(this.state.level + 1);
    const xpInLevel = Math.max(0, this.state.xp - currentLvlXP);
    const xpNeeded = Math.max(1, nextLvlXP - currentLvlXP);
    const xpPct = Math.min(100, Math.round((xpInLevel / xpNeeded) * 100));

    if (lvlEl) lvlEl.textContent = `Lvl ${this.state.level}`;
    if (titleEl) titleEl.textContent = this.state.title;
    if (xpBarEl) xpBarEl.style.width = `${xpPct}%`;
    if (xpTextEl) xpTextEl.textContent = `${this.state.xp} / ${nextLvlXP} XP`;
    if (satsEl) satsEl.textContent = `${this.state.satsBalance.toLocaleString()} Sats`;
    if (streakEl) streakEl.textContent = `${this.state.streakDays}d Streak`;

    // Quests screen profile elements
    const profName = document.getElementById('quest-prof-name');
    const profTitle = document.getElementById('quest-prof-title');
    const profLevel = document.getElementById('quest-prof-level');
    const profSats = document.getElementById('quest-prof-sats');
    const profVerif = document.getElementById('quest-prof-verifications');
    const profLitres = document.getElementById('quest-prof-litres');
    const profXpBar = document.getElementById('quest-prof-xp-bar');
    const profXpRatio = document.getElementById('quest-prof-xp-ratio');

    if (profName) profName.textContent = 'Otieno Richard (You)';
    if (profTitle) profTitle.textContent = this.state.title;
    if (profLevel) profLevel.textContent = `Level ${this.state.level}`;
    if (profSats) profSats.textContent = `${this.state.satsBalance.toLocaleString()} Sats`;
    if (profVerif) profVerif.textContent = `${this.state.stats.verificationsCount}`;
    if (profLitres) profLitres.textContent = `${this.state.stats.litresProtected.toLocaleString()} L`;
    if (profXpBar) profXpBar.style.width = `${xpPct}%`;
    if (profXpRatio) profXpRatio.textContent = `${xpInLevel} / ${xpNeeded} XP to Level ${this.state.level + 1}`;
  }

  // =========================================================================
  // Quests & Leaderboard Screen Rendering
  // =========================================================================

  renderQuestsScreen() {
    this.updateHUD();
    this.renderQuestsList();
    this.loadLeaderboard();
  }

  renderQuestsList() {
    const container = document.getElementById('quests-list-container');
    if (!container) return;

    const quests = Object.values(this.state.quests);
    container.innerHTML = quests.map(q => {
      const isComplete = q.current >= q.target;
      const pct = Math.min(100, Math.round((q.current / q.target) * 100));

      let actionBtn = '';
      if (q.claimed) {
        actionBtn = `<span class="bg-surface-low text-outline font-mono text-xs px-3 py-1.5 rounded-lg flex items-center gap-1 font-bold">✓ Claimed</span>`;
      } else if (isComplete) {
        actionBtn = `
          <button onclick="window.gameEngine.claimQuest('${q.id}')" class="bg-secondary hover:bg-secondary/90 text-white font-mono text-xs font-bold px-3 py-2 rounded-xl shadow-md transition-all animate-pulse cursor-pointer flex items-center gap-1">
            <span class="material-symbols-outlined text-[16px]">redeem</span> Claim +${q.sats} Sats
          </button>
        `;
      } else {
        actionBtn = `
          <button onclick="window.gameEngine.handleQuestAction('${q.id}')" class="bg-surface-high hover:bg-surface-low text-primary font-mono text-xs font-bold px-3 py-2 rounded-xl border border-surface-low transition-all cursor-pointer">
            Start Quest
          </button>
        `;
      }

      return `
        <div class="bg-white rounded-2xl p-4 border border-surface-low shadow-sm flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div class="space-y-1 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-headline font-bold text-sm text-primary">${escapeHtml(q.name)}</span>
              <span class="bg-secondary-container text-secondary text-[10px] font-mono font-bold px-2 py-0.5 rounded-full">+${q.xp} XP</span>
              <span class="bg-surface-high text-primary text-[10px] font-mono font-bold px-2 py-0.5 rounded-full">⚡ ${q.sats} Sats</span>
            </div>
            <p class="text-xs text-[#42474f]">${escapeHtml(q.desc)}</p>
            <div class="w-full max-w-md bg-surface-low rounded-full h-2 mt-2 overflow-hidden">
              <div class="bg-secondary h-2 rounded-full transition-all duration-500" style="width: ${pct}%;"></div>
            </div>
            <span class="text-[10px] font-mono text-outline">${q.current} / ${q.target} Completed (${pct}%)</span>
          </div>
          <div class="flex-shrink-0">
            ${actionBtn}
          </div>
        </div>
      `;
    }).join('');
  }

  handleQuestAction(questId) {
    if (questId === 'patrol') {
      window.switchTab('verify');
    } else if (questId === 'turbidity') {
      window.switchTab('report');
    } else if (questId === 'ai_sentry') {
      window.switchTab('verify');
    } else if (questId === 'blitz') {
      this.openGameModal();
    } else {
      window.switchTab('overview');
    }
  }

  async loadLeaderboard() {
    const container = document.getElementById('leaderboard-rows-container');
    const podiumEl = document.getElementById('leaderboard-podium-container');
    if (!container) return;

    let leaderboard = [];
    try {
      leaderboard = await api.getLeaderboard();
    } catch (e) {
      console.warn('[Gaming] Using baseline leaderboard:', e);
    }

    if (!Array.isArray(leaderboard) || leaderboard.length === 0) {
      leaderboard = [
        { id: 1, display_name: 'Wanja Rouwel', role: 'admin', tier: 'lake_guardian', reputation_score: 5.0, reports_count: 24, verifications_count: 89, total_sats: 4500 },
        { id: 2, display_name: 'Bernadette Akinyi', role: 'institution', tier: 'trusted_verifier', reputation_score: 4.8, reports_count: 19, verifications_count: 76, total_sats: 3800 },
        { id: 3, display_name: 'Otieno Richard', role: 'citizen', tier: 'senior_sentinel', reputation_score: 4.5, reports_count: 14, verifications_count: 38, total_sats: 1450 },
        { id: 4, display_name: 'Achieng Onyango (Dunga)', role: 'citizen', tier: 'water_scout', reputation_score: 4.2, reports_count: 9, verifications_count: 28, total_sats: 920 },
        { id: 5, display_name: 'Juma Mwangi (Kendu)', role: 'citizen', tier: 'water_scout', reputation_score: 3.9, reports_count: 7, verifications_count: 19, total_sats: 680 },
        { id: 6, display_name: 'Captain Brian Okoth', role: 'citizen', tier: 'water_scout', reputation_score: 3.7, reports_count: 6, verifications_count: 15, total_sats: 540 },
      ];
    }

    // Render Podium for Top 3
    if (podiumEl && leaderboard.length >= 3) {
      const top3 = [leaderboard[1], leaderboard[0], leaderboard[2]]; // 2nd, 1st, 3rd for podium layout
      const heights = ['h-32 bg-slate-200', 'h-40 bg-amber-100 border-2 border-amber-300', 'h-28 bg-amber-50'];
      const medals = ['🥈 2nd', '🥇 1st', '🥉 3rd'];

      podiumEl.innerHTML = top3.map((u, i) => {
        if (!u) return '';
        const isFirst = i === 1;
        return `
          <div class="flex-1 flex flex-col items-center">
            <div class="relative mb-2 flex flex-col items-center">
              <div class="w-12 h-12 rounded-full overflow-hidden border-2 ${isFirst ? 'border-amber-400 shadow-lg scale-110' : 'border-surface-high'} bg-surface-low flex items-center justify-center">
                <span class="material-symbols-outlined text-2xl text-primary">person</span>
              </div>
              <span class="absolute -top-2 bg-primary text-white font-mono text-[9px] px-1.5 py-0.2 rounded-full font-bold">${medals[i]}</span>
            </div>
            <span class="font-headline font-bold text-xs text-primary text-center truncate max-w-[100px]">${escapeHtml(u.display_name)}</span>
            <span class="font-mono text-[10px] text-secondary font-bold">⚡ ${Number(u.total_sats || 0).toLocaleString()} sats</span>
            <div class="w-full ${heights[i]} rounded-t-2xl mt-2 flex flex-col items-center justify-center font-headline font-bold text-primary text-lg">
              ${i === 1 ? '1' : (i === 0 ? '2' : '3')}
            </div>
          </div>
        `;
      }).join('');
    }

    // Render Table
    container.innerHTML = leaderboard.map((u, idx) => {
      const rank = idx + 1;
      const medalIcon = rank === 1 ? '🥇' : (rank === 2 ? '🥈' : (rank === 3 ? '🥉' : `#${rank}`));
      const isMe = u.display_name.includes('Otieno Richard');

      return `
        <div class="flex items-center justify-between p-3 rounded-xl ${isMe ? 'bg-surface-high border border-primary/20' : 'hover:bg-surface-low'} transition-colors text-xs font-mono">
          <div class="flex items-center gap-3">
            <span class="font-bold w-6 text-center text-sm">${medalIcon}</span>
            <div>
              <span class="font-headline font-bold text-primary block">${escapeHtml(u.display_name)} ${isMe ? '<span class="text-secondary font-mono text-[10px]">(You)</span>' : ''}</span>
              <span class="text-outline text-[10px]">⭐ ${(u.reputation_score || 4.5).toFixed(1)} Rep • ${u.verifications_count || 0} Verifications</span>
            </div>
          </div>
          <div class="text-right">
            <span class="font-bold text-secondary text-sm block">⚡ ${Number(u.total_sats || 0).toLocaleString()} sats</span>
            <span class="text-[10px] text-outline">${u.reports_count || 0} reports</span>
          </div>
        </div>
      `;
    }).join('');
  }

  // =========================================================================
  // "Lake Victoria Clean-Up Blitz" Mini-Game Logic
  // =========================================================================

  openGameModal() {
    const modal = document.getElementById('blitz-game-modal');
    if (modal) {
      modal.classList.remove('hidden');
      this.resetGameView();
    }
  }

  closeGameModal() {
    this.stopGame();
    const modal = document.getElementById('blitz-game-modal');
    if (modal) modal.classList.add('hidden');
  }

  resetGameView() {
    const startScreen = document.getElementById('game-start-screen');
    const playingScreen = document.getElementById('game-playing-screen');
    const overScreen = document.getElementById('game-over-screen');

    if (startScreen) startScreen.classList.remove('hidden');
    if (playingScreen) playingScreen.classList.add('hidden');
    if (overScreen) overScreen.classList.add('hidden');
  }

  startGame() {
    this.isGameRunning = true;
    this.gameScore = 0;
    this.gameTimeLeft = 30;
    this.gameCombo = 1.0;
    this.gameTargets = [];
    this.gameParticles = [];

    const startScreen = document.getElementById('game-start-screen');
    const playingScreen = document.getElementById('game-playing-screen');
    const overScreen = document.getElementById('game-over-screen');

    if (startScreen) startScreen.classList.add('hidden');
    if (overScreen) overScreen.classList.add('hidden');
    if (playingScreen) playingScreen.classList.remove('hidden');

    this.updateGameHUD();
    this.playBeep(880, 0.15, 'triangle');

    // Canvas init
    const canvas = document.getElementById('blitz-canvas');
    if (canvas) {
      canvas.width = canvas.parentElement.clientWidth || 600;
      canvas.height = canvas.parentElement.clientHeight || 380;
      canvas.onclick = (e) => this.handleCanvasClick(e);
    }

    // Spawn loop
    this.spawnTarget();
    this.spawnInterval = setInterval(() => {
      if (this.isGameRunning) this.spawnTarget();
    }, 750);

    // Timer loop
    this.timerInterval = setInterval(() => {
      if (!this.isGameRunning) return;
      this.gameTimeLeft -= 1;
      this.updateGameHUD();

      if (this.gameTimeLeft <= 5) {
        this.playBeep(440, 0.05, 'square');
      }

      if (this.gameTimeLeft <= 0) {
        this.endGame();
      }
    }, 1000);

    // Animation Loop
    this.lastFrameTime = performance.now();
    this.runGameLoop();
  }

  stopGame() {
    this.isGameRunning = false;
    if (this.spawnInterval) clearInterval(this.spawnInterval);
    if (this.timerInterval) clearInterval(this.timerInterval);
    if (this.gameLoopId) cancelAnimationFrame(this.gameLoopId);
  }

  spawnTarget() {
    const canvas = document.getElementById('blitz-canvas');
    if (!canvas) return;

    const types = [
      { type: 'spill', label: '🔴 Oil Spill', pts: 100, color: '#ba1a1a', radius: 24, duration: 3200 },
      { type: 'algae', label: '🟢 Algae Bloom', pts: 60, color: '#007354', radius: 22, duration: 3800 },
      { type: 'silt', label: '🟤 Turbidity Runoff', pts: 40, color: '#854d0e', radius: 20, duration: 4200 },
      { type: 'sats', label: '⚡ Sat Pouch', pts: 200, color: '#f59e0b', radius: 18, duration: 2500 },
    ];

    // Pick type with weighted probability
    const rand = Math.random();
    let selectedType = types[0];
    if (rand < 0.15) selectedType = types[3]; // 15% Sat pouch
    else if (rand < 0.45) selectedType = types[1]; // 30% Algae
    else if (rand < 0.75) selectedType = types[2]; // 30% Silt
    else selectedType = types[0]; // 25% Oil spill

    const padding = 40;
    const x = padding + Math.random() * (canvas.width - padding * 2);
    const y = padding + Math.random() * (canvas.height - padding * 2);

    this.gameTargets.push({
      ...selectedType,
      id: Math.random().toString(),
      x,
      y,
      createdAt: performance.now(),
      maxDuration: selectedType.duration,
      alpha: 1.0,
      scale: 0.1,
    });
  }

  handleCanvasClick(event) {
    if (!this.isGameRunning) return;

    const canvas = document.getElementById('blitz-canvas');
    if (!canvas) return;

    const rect = canvas.getBoundingClientRect();
    const clickX = event.clientX - rect.left;
    const clickY = event.clientY - rect.top;

    let hit = false;
    for (let i = this.gameTargets.length - 1; i >= 0; i--) {
      const t = this.gameTargets[i];
      const dist = Math.hypot(clickX - t.x, clickY - t.y);

      if (dist <= t.radius * 1.5) {
        hit = true;
        const earned = Math.round(t.pts * this.gameCombo);
        this.gameScore += earned;
        this.gameCombo = Math.min(3.0, Number((this.gameCombo + 0.2).toFixed(1)));

        // Audio
        if (t.type === 'sats') {
          this.playZap();
        } else {
          this.playBeep(520 + Math.random() * 200, 0.08, 'sine');
        }

        // Particle explosion
        this.createExplosion(t.x, t.y, t.color, earned);

        // Remove target
        this.gameTargets.splice(i, 1);
        break;
      }
    }

    if (!hit) {
      this.gameCombo = 1.0;
      this.playBeep(220, 0.05, 'sawtooth');
    }

    this.updateGameHUD();
  }

  createExplosion(x, y, color, points) {
    // Floater text
    this.gameParticles.push({
      type: 'text',
      x,
      y,
      text: `+${points}`,
      color: '#ffffff',
      alpha: 1.0,
      vy: -1.5,
    });

    // Color particles
    for (let i = 0; i < 12; i++) {
      const angle = (Math.PI * 2 / 12) * i;
      const speed = 1.5 + Math.random() * 2.5;
      this.gameParticles.push({
        type: 'spark',
        x,
        y,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        color,
        radius: 3 + Math.random() * 3,
        alpha: 1.0,
        decay: 0.03 + Math.random() * 0.02,
      });
    }
  }

  runGameLoop() {
    if (!this.isGameRunning) return;

    const now = performance.now();
    const canvas = document.getElementById('blitz-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Radar scan grid
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
    ctx.lineWidth = 1;
    for (let x = 0; x < canvas.width; x += 40) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, canvas.height);
      ctx.stroke();
    }
    for (let y = 0; y < canvas.height; y += 40) {
      ctx.beginPath();
      ctx.moveTo(y, 0);
      ctx.lineTo(canvas.width, y);
      ctx.stroke();
    }

    // Radar sweep line
    const sweepAngle = (now / 1500) % (Math.PI * 2);
    const cx = canvas.width / 2;
    const cy = canvas.height / 2;
    ctx.strokeStyle = 'rgba(0, 245, 160, 0.2)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(cx + Math.cos(sweepAngle) * 350, cy + Math.sin(sweepAngle) * 350);
    ctx.stroke();

    // Render and update targets
    for (let i = this.gameTargets.length - 1; i >= 0; i--) {
      const t = this.gameTargets[i];
      const age = now - t.createdAt;
      if (age >= t.maxDuration) {
        this.gameTargets.splice(i, 1);
        continue;
      }

      // Grow & Pulse
      const progress = age / t.maxDuration;
      t.scale = Math.min(1.0, age / 200);
      t.alpha = 1.0 - Math.pow(progress, 3);

      ctx.save();
      ctx.globalAlpha = t.alpha;

      // Glow ring
      ctx.beginPath();
      ctx.arc(t.x, t.y, t.radius * (1 + (Math.sin(now / 150) * 0.15)), 0, Math.PI * 2);
      ctx.fillStyle = t.color;
      ctx.shadowColor = t.color;
      ctx.shadowBlur = 12;
      ctx.fill();

      // Inner Core
      ctx.beginPath();
      ctx.arc(t.x, t.y, t.radius * 0.5, 0, Math.PI * 2);
      ctx.fillStyle = '#ffffff';
      ctx.fill();

      // Label
      ctx.shadowBlur = 0;
      ctx.font = 'bold 11px "JetBrains Mono", monospace';
      ctx.fillStyle = '#ffffff';
      ctx.textAlign = 'center';
      ctx.fillText(t.label, t.x, t.y + t.radius + 14);

      ctx.restore();
    }

    // Render and update particles
    for (let i = this.gameParticles.length - 1; i >= 0; i--) {
      const p = this.gameParticles[i];
      p.alpha -= (p.decay || 0.02);

      if (p.alpha <= 0) {
        this.gameParticles.splice(i, 1);
        continue;
      }

      ctx.save();
      ctx.globalAlpha = Math.max(0, p.alpha);

      if (p.type === 'text') {
        p.y += p.vy;
        ctx.font = 'bold 14px "JetBrains Mono", monospace';
        ctx.fillStyle = '#fef08a';
        ctx.textAlign = 'center';
        ctx.shadowColor = '#000000';
        ctx.shadowBlur = 4;
        ctx.fillText(p.text, p.x, p.y);
      } else {
        p.x += p.vx;
        p.y += p.vy;
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
        ctx.fillStyle = p.color;
        ctx.fill();
      }

      ctx.restore();
    }

    this.gameLoopId = requestAnimationFrame(() => this.runGameLoop());
  }

  updateGameHUD() {
    const scoreEl = document.getElementById('blitz-hud-score');
    const timerEl = document.getElementById('blitz-hud-timer');
    const comboEl = document.getElementById('blitz-hud-combo');

    if (scoreEl) scoreEl.textContent = this.gameScore.toLocaleString();
    if (timerEl) timerEl.textContent = `${this.gameTimeLeft}s`;
    if (comboEl) comboEl.textContent = `x${this.gameCombo.toFixed(1)}`;
  }

  endGame() {
    this.stopGame();
    this.playLevelUp();

    const playingScreen = document.getElementById('game-playing-screen');
    const overScreen = document.getElementById('game-over-screen');

    if (playingScreen) playingScreen.classList.add('hidden');
    if (overScreen) overScreen.classList.remove('hidden');

    const finalScoreEl = document.getElementById('blitz-final-score');
    const xpEarnedEl = document.getElementById('blitz-final-xp');
    const satsEarnedEl = document.getElementById('blitz-final-sats');
    const rankGradeEl = document.getElementById('blitz-final-grade');

    const xpEarned = Math.round(this.gameScore * 0.4);
    const satsEarned = Math.max(15, Math.round(this.gameScore * 0.12));

    let grade = 'B';
    if (this.gameScore >= 700) grade = 'S+';
    else if (this.gameScore >= 500) grade = 'A';
    else if (this.gameScore >= 300) grade = 'B';
    else grade = 'C';

    if (finalScoreEl) finalScoreEl.textContent = this.gameScore.toLocaleString();
    if (xpEarnedEl) xpEarnedEl.textContent = `+${xpEarned} XP`;
    if (satsEarnedEl) satsEarnedEl.textContent = `⚡ +${satsEarned} SATS`;
    if (rankGradeEl) rankGradeEl.textContent = `Rank: ${grade}`;

    this.pendingPayout = { xp: xpEarned, sats: satsEarned };

    // Update quest progress
    if (this.gameScore >= 400) {
      this.state.quests.blitz.current = this.gameScore;
      this.saveState();
    }
  }

  claimMiniGamePayout() {
    if (!this.pendingPayout) return;
    const { xp, sats } = this.pendingPayout;

    this.addXP(xp, 'Clean-Up Blitz Performance');
    this.addSats(sats, 'Clean-Up Blitz Bounty');
    this.triggerConfetti();

    this.pendingPayout = null;
    this.closeGameModal();
    this.renderQuestsScreen();
  }

  // =========================================================================
  // Visual Effects & Toasts
  // =========================================================================

  showToast(message, type = 'info') {
    let container = document.getElementById('game-toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'game-toast-container';
      container.className = 'fixed bottom-20 right-6 z-50 flex flex-col gap-2 pointer-events-none';
      document.body.appendChild(container);
    }

    const toast = document.createElement('div');
    const bg = type === 'success' ? 'bg-[#007354] text-white' : (type === 'lightning' ? 'bg-amber-500 text-black font-bold' : 'bg-primary text-white');
    toast.className = `${bg} px-4 py-3 rounded-2xl shadow-xl font-mono text-xs flex items-center gap-2 transform translate-y-4 opacity-0 transition-all duration-300 pointer-events-auto`;
    toast.innerHTML = `<span>${message}</span>`;
    container.appendChild(toast);

    setTimeout(() => {
      toast.classList.remove('translate-y-4', 'opacity-0');
    }, 10);

    setTimeout(() => {
      toast.classList.add('translate-y-4', 'opacity-0');
      setTimeout(() => toast.remove(), 300);
    }, 4500);
  }

  triggerConfetti() {
    // Simple confetti particle shower on canvas overlay
    const canvas = document.createElement('canvas');
    canvas.style.position = 'fixed';
    canvas.style.top = '0';
    canvas.style.left = '0';
    canvas.style.width = '100vw';
    canvas.style.height = '100vh';
    canvas.style.pointerEvents = 'none';
    canvas.style.zIndex = '9999';
    document.body.appendChild(canvas);

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
    const ctx = canvas.getContext('2d');

    const particles = [];
    const colors = ['#007354', '#f59e0b', '#0d3b66', '#ba1a1a', '#99f5cd'];
    for (let i = 0; i < 60; i++) {
      particles.push({
        x: canvas.width / 2,
        y: canvas.height / 2,
        vx: (Math.random() - 0.5) * 16,
        vy: (Math.random() - 0.7) * 18,
        color: colors[Math.floor(Math.random() * colors.length)],
        radius: 4 + Math.random() * 5,
        alpha: 1.0,
      });
    }

    let frames = 0;
    const animate = () => {
      frames++;
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      particles.forEach(p => {
        p.x += p.vx;
        p.y += p.vy;
        p.vy += 0.35; // gravity
        p.alpha -= 0.015;

        ctx.save();
        ctx.globalAlpha = Math.max(0, p.alpha);
        ctx.fillStyle = p.color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
      });

      if (frames < 70) {
        requestAnimationFrame(animate);
      } else {
        canvas.remove();
      }
    };
    animate();
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>"']/g, (m) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[m]));
}

export const gameEngine = new GameEngine();
window.gameEngine = gameEngine;

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => gameEngine.init());
} else {
  gameEngine.init();
}
