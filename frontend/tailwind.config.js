import defaultTheme from 'tailwindcss/defaultTheme';

/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class',
	theme: {
		extend: {
			colors: {
				primary: '#FF9549', // Sandy Brown
				'sandy-brown': '#FF9549',
				'cool-steel': '#929A98',
				'carbon-black': '#1C2327',
				'ash-grey': '#BEC6C3',
				black: '#000000',
				'background-light': '#F5F6F5',
				'background-dark': '#1C2327'
			},
			fontFamily: {
				sans: ['Space Grotesk', ...defaultTheme.fontFamily.sans],
				display: ['Syncopate', 'Space Grotesk', 'sans-serif'],
				mono: ['JetBrains Mono', ...defaultTheme.fontFamily.mono]
			},
			borderRadius: {
				DEFAULT: '4px',
				xl: '24px'
			}
		}
	},
	plugins: []
};
