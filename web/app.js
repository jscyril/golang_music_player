// ===== GTMPC Web UI — Application Logic =====

(function () {
    'use strict';

    // --- State ---
    const state = {
        token: localStorage.getItem('gtmpc_token') || null,
        username: localStorage.getItem('gtmpc_user') || null,
        tracks: [],
        filteredTracks: [],
        playlists: [],
        activePlaylist: null,
        currentTrackIndex: -1,
        isPlaying: false,
    };

    // --- DOM refs ---
    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    const dom = {
        authScreen: $('#auth-screen'),
        appScreen: $('#app-screen'),
        authError: $('#auth-error'),
        loginForm: $('#login-form'),
        registerForm: $('#register-form'),
        trackList: $('#track-list'),
        emptyState: $('#empty-state'),
        loadingState: $('#loading-state'),
        searchInput: $('#search-input'),
        userDisplay: $('#user-display'),
        trackCount: $('#track-count'),
        libraryTitle: $('#library-title'),
        libraryNavBtn: $('#library-nav-btn'),
        playlistNavList: $('#playlist-nav-list'),
        createPlaylistForm: $('#create-playlist-form'),
        playlistNameInput: $('#playlist-name-input'),
        playlistTarget: $('#playlist-target'),
        deletePlaylistBtn: $('#delete-playlist-btn'),
        playerBar: $('#player-bar'),
        playerTitle: $('#player-title'),
        playerArtist: $('#player-artist'),
        btnPlay: $('#btn-play'),
        btnPrev: $('#btn-prev'),
        btnNext: $('#btn-next'),
        iconPlay: $('#icon-play'),
        iconPause: $('#icon-pause'),
        seekBar: $('#seek-bar'),
        volumeBar: $('#volume-bar'),
        currentTime: $('#current-time'),
        totalTime: $('#total-time'),
        audio: $('#audio-element'),
    };

    // ===== API CLIENT =====
    const api = {
        base: '',

        async request(method, path, body) {
            const headers = { 'Content-Type': 'application/json' };
            if (state.token) {
                headers['Authorization'] = `Bearer ${state.token}`;
            }
            const opts = { method, headers };
            if (body) opts.body = JSON.stringify(body);

            const res = await fetch(this.base + path, opts);
            const data = await res.json();

            if (!res.ok || !data.success) {
                throw new Error(data.error || `Request failed (${res.status})`);
            }
            return data.data;
        },

        register(username, password, role) {
            return this.request('POST', '/api/auth/register', { username, password, role });
        },

        login(username, password) {
            return this.request('POST', '/api/auth/login', { username, password });
        },

        getTracks() {
            return this.request('GET', '/api/library/tracks');
        },

        searchTracks(q) {
            return this.request('GET', `/api/library/search?q=${encodeURIComponent(q)}`);
        },

        getPlaylists() {
            return this.request('GET', '/api/playlists');
        },

        createPlaylist(name, description = '') {
            return this.request('POST', '/api/playlists', { name, description });
        },

        getPlaylist(id) {
            return this.request('GET', `/api/playlists/${encodeURIComponent(id)}`);
        },

        deletePlaylist(id) {
            return this.request('DELETE', `/api/playlists/${encodeURIComponent(id)}`);
        },

        addTrackToPlaylist(playlistId, trackId) {
            return this.request('POST', `/api/playlists/${encodeURIComponent(playlistId)}/tracks`, { track_id: trackId });
        },

        removeTrackFromPlaylist(playlistId, trackId) {
            return this.request('DELETE', `/api/playlists/${encodeURIComponent(playlistId)}/tracks/${encodeURIComponent(trackId)}`);
        },

        streamUrl(trackId) {
            return `${this.base}/api/stream/${trackId}`;
        },
    };

    // ===== AUTH =====
    function showScreen(screen) {
        dom.authScreen.classList.remove('active');
        dom.appScreen.classList.remove('active');
        screen.classList.add('active');
    }

    function showAuthError(msg) {
        dom.authError.textContent = msg;
        dom.authError.classList.remove('hidden');
    }

    function clearAuthError() {
        dom.authError.classList.add('hidden');
    }

    // Tab switching
    $$('.auth-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            $$('.auth-tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            clearAuthError();
            if (tab.dataset.tab === 'login') {
                dom.loginForm.classList.remove('hidden');
                dom.registerForm.classList.add('hidden');
            } else {
                dom.loginForm.classList.add('hidden');
                dom.registerForm.classList.remove('hidden');
            }
        });
    });

    // Login
    dom.loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        clearAuthError();
        const username = $('#login-username').value.trim();
        const password = $('#login-password').value;
        try {
            const data = await api.login(username, password);
            state.token = data.token;
            state.username = data.user.username;
            localStorage.setItem('gtmpc_token', state.token);
            localStorage.setItem('gtmpc_user', state.username);
            enterApp();
        } catch (err) {
            showAuthError(err.message);
        }
    });

    // Register
    dom.registerForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        clearAuthError();
        const username = $('#reg-username').value.trim();
        const password = $('#reg-password').value;
        try {
            await api.register(username, password, 'user');
            // Auto-login after registration
            const data = await api.login(username, password);
            state.token = data.token;
            state.username = data.user.username;
            localStorage.setItem('gtmpc_token', state.token);
            localStorage.setItem('gtmpc_user', state.username);
            enterApp();
        } catch (err) {
            showAuthError(err.message);
        }
    });

    // Logout
    $('#logout-btn').addEventListener('click', () => {
        state.token = null;
        state.username = null;
        state.activePlaylist = null;
        localStorage.removeItem('gtmpc_token');
        localStorage.removeItem('gtmpc_user');
        dom.audio.pause();
        dom.audio.src = '';
        showScreen(dom.authScreen);
    });

    // ===== LIBRARY =====
    async function enterApp() {
        showScreen(dom.appScreen);
        dom.userDisplay.textContent = state.username;
        dom.loadingState.classList.remove('hidden');
        dom.trackList.classList.add('hidden');
        dom.emptyState.classList.add('hidden');

        try {
            const [tracks, playlists] = await Promise.all([
                api.getTracks(),
                api.getPlaylists(),
            ]);
            state.tracks = tracks || [];
            state.playlists = playlists || [];
            state.activePlaylist = null;
            applyCurrentFilter();
            renderPlaylists();
        } catch (err) {
            // Token expired — go back to login
            if (err.message.includes('token') || err.message.includes('authorization')) {
                state.token = null;
                localStorage.removeItem('gtmpc_token');
                showScreen(dom.authScreen);
                showAuthError('Session expired. Please login again.');
            } else {
                dom.loadingState.classList.add('hidden');
                dom.emptyState.classList.remove('hidden');
            }
        }
    }

    function getSourceTracks() {
        return state.activePlaylist ? (state.activePlaylist.tracks || []) : state.tracks;
    }

    function applyCurrentFilter() {
        const query = dom.searchInput.value.trim();
        const source = getSourceTracks();

        if (query === '') {
            state.filteredTracks = source;
            dom.libraryTitle.textContent = state.activePlaylist ? state.activePlaylist.name : 'Your Library';
        } else {
            const q = query.toLowerCase();
            state.filteredTracks = source.filter(t =>
                (t.title && t.title.toLowerCase().includes(q)) ||
                (t.artist && t.artist.toLowerCase().includes(q)) ||
                (t.album && t.album.toLowerCase().includes(q))
            );
            dom.libraryTitle.textContent = `Search: "${query}"`;
        }

        dom.deletePlaylistBtn.classList.toggle('hidden', !state.activePlaylist);
        dom.playlistTarget.classList.toggle('hidden', state.activePlaylist || state.playlists.length === 0);
        dom.uploadBtn.classList.toggle('hidden', !!state.activePlaylist);
        renderTracks(state.filteredTracks);
    }

    function renderTracks(tracks) {
        dom.loadingState.classList.add('hidden');

        if (!tracks || tracks.length === 0) {
            dom.trackList.classList.add('hidden');
            dom.emptyState.classList.remove('hidden');
            dom.trackCount.textContent = '';
            return;
        }

        dom.emptyState.classList.add('hidden');
        dom.trackList.classList.remove('hidden');
        dom.trackCount.textContent = `${tracks.length} track${tracks.length !== 1 ? 's' : ''}`;

        // Build header + rows
        const actionLabel = state.activePlaylist ? 'Remove' : 'Add';
        let html = `<div class="track-list-header">
            <span>#</span><span>Title</span><span>Album</span><span>Duration</span><span></span>
        </div>`;

        tracks.forEach((track, i) => {
            const isPlaying = state.currentTrackIndex === i && state.isPlaying;
            const dur = formatDuration(track.duration);
            const artist = track.artist || 'Unknown Artist';
            const title = track.title || 'Unknown Title';
            const album = track.album || '—';

            html += `<div class="track-row${isPlaying ? ' playing' : ''}" data-index="${i}" data-track-id="${escapeHtml(track.id)}">
                <span class="track-number">${isPlaying ? '♫' : i + 1}</span>
                <div class="track-info">
                    <span class="track-title">${escapeHtml(title)}</span>
                    <span class="track-artist-inline">${escapeHtml(artist)}</span>
                </div>
                <span class="track-album">${escapeHtml(album)}</span>
                <span class="track-duration">${dur}</span>
                <button class="track-action-btn" data-track-id="${escapeHtml(track.id)}">${actionLabel}</button>
            </div>`;
        });

        dom.trackList.innerHTML = html;

        // Click handlers
        dom.trackList.querySelectorAll('.track-row').forEach(row => {
            row.addEventListener('click', () => {
                const idx = parseInt(row.dataset.index, 10);
                playTrack(idx);
            });
        });

        dom.trackList.querySelectorAll('.track-action-btn').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                e.stopPropagation();
                if (state.activePlaylist) {
                    await removeFromActivePlaylist(btn.dataset.trackId);
                } else {
                    await addToSelectedPlaylist(btn.dataset.trackId);
                }
            });
        });
    }

    function renderPlaylists() {
        dom.libraryNavBtn.classList.toggle('active', !state.activePlaylist);
        dom.playlistNavList.innerHTML = '';

        state.playlists.forEach(pl => {
            const btn = document.createElement('button');
            btn.className = `playlist-nav${state.activePlaylist && state.activePlaylist.id === pl.id ? ' active' : ''}`;
            btn.textContent = `${pl.name} (${(pl.tracks || []).length})`;
            btn.addEventListener('click', () => openPlaylist(pl.id));
            dom.playlistNavList.appendChild(btn);
        });

        dom.playlistTarget.innerHTML = state.playlists.map(pl =>
            `<option value="${escapeHtml(pl.id)}">${escapeHtml(pl.name)}</option>`
        ).join('');
    }

    async function refreshPlaylists() {
        state.playlists = await api.getPlaylists() || [];
        if (state.activePlaylist) {
            const stillExists = state.playlists.some(pl => pl.id === state.activePlaylist.id);
            state.activePlaylist = stillExists ? await api.getPlaylist(state.activePlaylist.id) : null;
        }
        renderPlaylists();
        applyCurrentFilter();
    }

    async function openPlaylist(id) {
        state.activePlaylist = await api.getPlaylist(id);
        dom.searchInput.value = '';
        state.currentTrackIndex = -1;
        renderPlaylists();
        applyCurrentFilter();
    }

    async function addToSelectedPlaylist(trackId) {
        const playlistId = dom.playlistTarget.value;
        if (!playlistId) return;
        await api.addTrackToPlaylist(playlistId, trackId);
        await refreshPlaylists();
    }

    async function removeFromActivePlaylist(trackId) {
        if (!state.activePlaylist) return;
        state.activePlaylist = await api.removeTrackFromPlaylist(state.activePlaylist.id, trackId);
        state.playlists = state.playlists.map(pl => pl.id === state.activePlaylist.id ? state.activePlaylist : pl);
        renderPlaylists();
        applyCurrentFilter();
    }

    // ===== SEARCH =====
    let searchTimeout = null;
    dom.searchInput.addEventListener('input', () => {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            applyCurrentFilter();
        }, 200);
    });

    dom.libraryNavBtn.addEventListener('click', () => {
        state.activePlaylist = null;
        state.currentTrackIndex = -1;
        dom.searchInput.value = '';
        renderPlaylists();
        applyCurrentFilter();
    });

    dom.createPlaylistForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const name = dom.playlistNameInput.value.trim();
        if (!name) return;
        await api.createPlaylist(name);
        dom.playlistNameInput.value = '';
        await refreshPlaylists();
    });

    dom.deletePlaylistBtn.addEventListener('click', async () => {
        if (!state.activePlaylist) return;
        await api.deletePlaylist(state.activePlaylist.id);
        state.activePlaylist = null;
        await refreshPlaylists();
    });

    // ===== UPLOAD =====
    const uploadBtn = $('#upload-btn');
    const fileInput = $('#file-input');

    uploadBtn.addEventListener('click', () => fileInput.click());

    fileInput.addEventListener('change', async () => {
        const files = fileInput.files;
        if (!files || files.length === 0) return;

        uploadBtn.disabled = true;
        uploadBtn.innerHTML = '<span class="spinner" style="width:16px;height:16px;border-width:2px;margin:0"></span> Uploading...';

        let uploaded = 0;
        for (const file of files) {
            try {
                const formData = new FormData();
                formData.append('file', file);

                const res = await fetch('/api/library/upload', {
                    method: 'POST',
                    headers: { 'Authorization': `Bearer ${state.token}` },
                    body: formData,
                });
                const data = await res.json();
                if (data.success) {
                    uploaded++;
                }
            } catch (err) {
                console.error('Upload failed for', file.name, err);
            }
        }

        // Reset input and button
        fileInput.value = '';
        uploadBtn.disabled = false;
        uploadBtn.innerHTML = '<svg viewBox="0 0 24 24" width="16" height="16"><path d="M21,15v4a2,2,0,0,1-2,2H5a2,2,0,0,1-2-2V15" stroke="currentColor" stroke-width="2" fill="none"/><polyline points="17,8 12,3 7,8" stroke="currentColor" stroke-width="2" fill="none"/><line x1="12" y1="3" x2="12" y2="15" stroke="currentColor" stroke-width="2"/></svg> Upload Music';

        if (uploaded > 0) {
            // Refresh library
            const tracks = await api.getTracks();
            state.tracks = tracks || [];
            applyCurrentFilter();
        }
    });

    // ===== AUDIO PLAYER =====
    function playTrack(index) {
        if (index < 0 || index >= state.filteredTracks.length) return;

        const track = state.filteredTracks[index];
        state.currentTrackIndex = index;

        // Set audio source with auth header workaround:
        // We fetch the stream with fetch + auth, create a blob URL
        fetchAndPlayStream(track);

        // Update player bar UI
        dom.playerBar.classList.remove('hidden');
        dom.playerTitle.textContent = track.title || 'Unknown';
        dom.playerArtist.textContent = track.artist || 'Unknown Artist';

        // Re-render to highlight the playing track
        renderTracks(state.filteredTracks);
    }

    async function fetchAndPlayStream(track) {
        try {
            const res = await fetch(api.streamUrl(track.id), {
                headers: { 'Authorization': `Bearer ${state.token}` },
            });

            if (!res.ok) throw new Error('Stream failed');

            const blob = await res.blob();
            const url = URL.createObjectURL(blob);

            // Revoke previous blob URL to free memory
            if (dom.audio.src && dom.audio.src.startsWith('blob:')) {
                URL.revokeObjectURL(dom.audio.src);
            }

            dom.audio.src = url;
            await dom.audio.play();
            state.isPlaying = true;
        } catch (err) {
            console.error('Stream error:', err);
            state.isPlaying = false;
        } finally {
            updatePlayButton();
            renderTracks(state.filteredTracks);
        }
    }

    function updatePlayButton() {
        if (state.isPlaying) {
            dom.iconPlay.classList.add('hidden');
            dom.iconPause.classList.remove('hidden');
        } else {
            dom.iconPlay.classList.remove('hidden');
            dom.iconPause.classList.add('hidden');
        }
    }

    // Play/Pause toggle
    dom.btnPlay.addEventListener('click', () => {
        if (!dom.audio.src) return;
        if (state.isPlaying) {
            dom.audio.pause();
            state.isPlaying = false;
        } else {
            dom.audio.play().then(() => {
                state.isPlaying = true;
                updatePlayButton();
                renderTracks(state.filteredTracks);
            }).catch(err => console.error('Play failed:', err));
            return;
        }
        updatePlayButton();
        renderTracks(state.filteredTracks);
    });

    // Next / Previous
    dom.btnNext.addEventListener('click', () => {
        if (state.currentTrackIndex < state.filteredTracks.length - 1) {
            playTrack(state.currentTrackIndex + 1);
        }
    });

    dom.btnPrev.addEventListener('click', () => {
        if (state.currentTrackIndex > 0) {
            playTrack(state.currentTrackIndex - 1);
        }
    });

    // Seek bar
    dom.audio.addEventListener('timeupdate', () => {
        if (dom.audio.duration && !isNaN(dom.audio.duration)) {
            const pct = (dom.audio.currentTime / dom.audio.duration) * 100;
            dom.seekBar.value = pct;
            dom.currentTime.textContent = formatTime(dom.audio.currentTime);
            dom.totalTime.textContent = formatTime(dom.audio.duration);

            // Update the filled portion of the slider
            dom.seekBar.style.background = `linear-gradient(to right, #7c5cfc ${pct}%, #2a2a3a ${pct}%)`;
        }
    });

    dom.seekBar.addEventListener('input', () => {
        if (dom.audio.duration) {
            dom.audio.currentTime = (dom.seekBar.value / 100) * dom.audio.duration;
        }
    });

    // Volume
    dom.volumeBar.addEventListener('input', () => {
        dom.audio.volume = dom.volumeBar.value / 100;
        const pct = dom.volumeBar.value;
        dom.volumeBar.style.background = `linear-gradient(to right, #7c5cfc ${pct}%, #2a2a3a ${pct}%)`;
    });
    // Initialize volume
    dom.audio.volume = 0.8;
    dom.volumeBar.style.background = `linear-gradient(to right, #7c5cfc 80%, #2a2a3a 80%)`;

    // Track ended — auto-next
    dom.audio.addEventListener('ended', () => {
        state.isPlaying = false;
        updatePlayButton();
        if (state.currentTrackIndex < state.filteredTracks.length - 1) {
            playTrack(state.currentTrackIndex + 1);
        } else {
            renderTracks(state.filteredTracks);
        }
    });

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
        // Don't intercept when typing in inputs
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
        if (!dom.appScreen.classList.contains('active')) return;

        switch (e.code) {
            case 'Space':
                e.preventDefault();
                dom.btnPlay.click();
                break;
            case 'ArrowRight':
                if (dom.audio.src) dom.audio.currentTime = Math.min(dom.audio.currentTime + 5, dom.audio.duration || 0);
                break;
            case 'ArrowLeft':
                if (dom.audio.src) dom.audio.currentTime = Math.max(dom.audio.currentTime - 5, 0);
                break;
            case 'KeyN':
                dom.btnNext.click();
                break;
            case 'KeyP':
                dom.btnPrev.click();
                break;
        }
    });

    // ===== HELPERS =====
    function formatDuration(nanos) {
        // Go's time.Duration is in nanoseconds
        const totalSeconds = Math.floor(nanos / 1e9);
        if (totalSeconds <= 0) return '—';
        const min = Math.floor(totalSeconds / 60);
        const sec = totalSeconds % 60;
        return `${min}:${sec.toString().padStart(2, '0')}`;
    }

    function formatTime(seconds) {
        if (!seconds || isNaN(seconds)) return '0:00';
        const min = Math.floor(seconds / 60);
        const sec = Math.floor(seconds % 60);
        return `${min}:${sec.toString().padStart(2, '0')}`;
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // ===== INIT =====
    // If we have a saved token, try to enter the app directly
    if (state.token) {
        enterApp();
    } else {
        showScreen(dom.authScreen);
    }
})();
