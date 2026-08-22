/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/**/*.templ", "./assets/css/**/*.css"],
  theme: {
    extend: {
      colors: {
        // ink — navy text hierarchy (from reference screenshots: ~#001020 body, #103040 headings)
        ink: {
          950: "#001020",
          900: "#0B1B2B",
          800: "#103040",
          700: "#223A4C",
          600: "#3C4C5C",
          500: "#55677A",
          400: "#7C8B98",
        },
        // steel — primary accent (buttons, links, focus). Reference accent ≈ #4090C0.
        steel: {
          50: "#F0F6FA",
          100: "#DCEAF4",
          200: "#B9D6E9",
          300: "#8FBDD9",
          400: "#63A3C9",
          500: "#4090C0",
          600: "#347CA8",
          700: "#2B668A",
          800: "#23516D",
          900: "#1C4158",
        },
        // chrome — topbar (dark slate) + sidebar (light gray) family
        chrome: {
          50: "#F3F5F6",
          100: "#E3E7EA",
          200: "#D0D6DA",
          300: "#B3BCC3",
          400: "#8B98A1",
          500: "#5E6B76",
          600: "#4A5A66",
          700: "#3A4B56",
          800: "#2F3E48",
          900: "#22303A",
        },
        // canvas — page backgrounds (gray #EEEFEF, blue-tinted #F2F9FB on the dashboard)
        canvas: {
          DEFAULT: "#EEEFEF",
          blue: "#F2F9FB",
        },
      },
      fontFamily: {
        sans: [
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "Segoe UI",
          "Roboto",
          "Helvetica Neue",
          "Arial",
          "sans-serif",
        ],
      },
      boxShadow: {
        card: "0 1px 3px 0 rgba(0, 16, 32, 0.08), 0 1px 2px -1px rgba(0, 16, 32, 0.06)",
      },
    },
  },
  plugins: [],
};
