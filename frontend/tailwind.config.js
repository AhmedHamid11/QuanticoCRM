import defaultTheme from 'tailwindcss/defaultTheme';

/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class',
	theme: {
		extend: {
			colors: {
				primary: '#FFD468', // Mustard
				'princeton-orange': '#FF9145',
				'grey-olive': '#7F898E',
				silver: '#C4CCC9',
				'silver-2': '#C1CAC8',
				black: '#000000',
				'background-light': '#F2F4F3',
				'background-dark': '#0A0B0B'
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
