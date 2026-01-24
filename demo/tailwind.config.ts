import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        slate: {
          850: "#1a2332",
          950: "#0c1220",
        },
      },
    },
  },
  plugins: [],
};
export default config;
