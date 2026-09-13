import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { initTheme } from './lib/theme.js'

// Before first paint, so the app never flashes dark-then-light (or vice
// versa) on startup.
initTheme()

const target = document.getElementById('app')
if (!target) throw new Error('Missing #app mount point')

const app = mount(App, { target })

export default app
