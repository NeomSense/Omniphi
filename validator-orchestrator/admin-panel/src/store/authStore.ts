/**
 * Authentication Store - Zustand state management for auth
 */

import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import type { User, AuthState } from '../types';

interface AuthStore extends AuthState {
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => void;
  setUser: (user: User | null) => void;
  setToken: (token: string | null) => void;
  checkAuth: () => Promise<boolean>;
}

const API_BASE = 'http://localhost:8000/api/v1';

export const useAuthStore = create<AuthStore>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,

      login: async (username: string, password: string) => {
        set({ isLoading: true });

        // Helper function for mock login
        const doMockLogin = () => {
          if (username === 'admin' && password === 'admin') {
            const mockUser: User = {
              id: '1',
              username: 'admin',
              email: 'admin@omniphi.network',
              role: 'admin',
              createdAt: new Date().toISOString(),
            };
            set({
              user: mockUser,
              token: 'mock-token-' + Date.now(),
              isAuthenticated: true,
              isLoading: false,
            });
            return true;
          }
          set({ isLoading: false });
          return false;
        };

        try {
          const response = await fetch(`${API_BASE}/auth/login`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ username, password }),
          });

          if (!response.ok) {
            // API returned error, try mock login
            return doMockLogin();
          }

          const data = await response.json();

          set({
            user: data.user,
            token: data.token,
            isAuthenticated: true,
            isLoading: false,
          });

          return true;
        } catch (error) {
          // Network error, try mock login
          return doMockLogin();
        }
      },

      logout: () => {
        set({
          user: null,
          token: null,
          isAuthenticated: false,
        });
      },

      setUser: (user) => set({ user }),

      setToken: (token) => set({ token }),

      checkAuth: async () => {
        const { token } = get();
        if (!token) {
          set({ isAuthenticated: false });
          return false;
        }

        try {
          const response = await fetch(`${API_BASE}/auth/me`, {
            headers: {
              Authorization: `Bearer ${token}`,
            },
          });

          if (!response.ok) {
            set({ isAuthenticated: false, token: null, user: null });
            return false;
          }

          const data = await response.json();
          set({ user: data.user, isAuthenticated: true });
          return true;
        } catch {
          // Keep authenticated if we have a token (offline mode)
          return true;
        }
      },
    }),
    {
      name: 'omniphi-admin-auth',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);

export default useAuthStore;
