import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./index.html",
    "./src/**/*.{svelte,js,ts,jsx,tsx}",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // Midnight Platform Palette
        obsidian: {
          DEFAULT: "#0a0b10",
          card: "#12131c",
          elevated: "#181a26",
          border: "rgba(255, 255, 255, 0.08)",
        },
        "neo-obsidian": {
          DEFAULT: "#0a0b10",
          card: "#12131c",
          elevated: "#181a26",
          border: "rgba(255, 255, 255, 0.08)",
        },
        violet: {
          DEFAULT: "#7c3aed",
          light: "#8b5cf6",
          glow: "rgba(124, 58, 237, 0.35)",
        },
        cyan: {
          DEFAULT: "#06b6d4",
          bright: "#00f5ff",
          glow: "rgba(6, 182, 212, 0.35)",
        },
        "electric-violet": {
          DEFAULT: "#7c3aed",
          light: "#8b5cf6",
          glow: "rgba(124, 58, 237, 0.35)",
        },
        "neon-cyan": {
          DEFAULT: "#06b6d4",
          bright: "#00f5ff",
          glow: "rgba(6, 182, 212, 0.35)",
        },
        "dark-glass": {
          DEFAULT: "rgba(10, 11, 16, 0.75)",
          border: "rgba(255, 255, 255, 0.08)",
        },
        neo: {
          obsidian: "#0a0b10",
          card: "#12131c",
          elevated: "#181a26",
          violet: "#7c3aed",
          cyan: "#06b6d4",
          glass: "rgba(10, 11, 16, 0.75)",
          border: "rgba(255, 255, 255, 0.08)",
        },
        status: {
          running: "#10b981",
          stopped: "#f43f5e",
          paused: "#f59e0b",
        },
      },
      boxShadow: {
        "glow-violet": "0 0 20px -3px rgba(124, 58, 237, 0.4)",
        "glow-cyan": "0 0 20px -3px rgba(6, 182, 212, 0.4)",
        "glow-emerald": "0 0 15px -3px rgba(16, 185, 129, 0.4)",
        "glow-rose": "0 0 15px -3px rgba(244, 63, 94, 0.4)",
        "glow-amber": "0 0 15px -3px rgba(245, 158, 11, 0.4)",
      },
    },
  },
  plugins: [],
};

export default config;
