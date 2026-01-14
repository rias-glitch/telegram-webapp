import { api } from './client'

export async function playCoinFlip(bet) {
  return api.post('/game/coinflip', { bet })
}

export async function playRPS(bet, move) {
  return api.post('/game/rps', { bet, move })
}

export async function playMines(bet, pick) {
  return api.post('/game/mines', { bet, pick })
}

export async function spinCase() {
  return api.post('/game/case', {})
}

export async function getMyGames() {
  return api.get('/me/games')
}

export async function getTopUsers() {
  return api.get('/top')
}

// Wheel game
export async function playWheel(bet) {
  return api.post('/game/wheel', { bet })
}

export async function getWheelInfo() {
  return api.get('/game/wheel/info')
}

// Dice game (1-6)
export async function playDice(bet, target) {
  return api.post('/game/dice', { bet, target })
}

export async function getDiceInfo() {
  return api.get('/game/dice/info')
}

// Mines Pro game
export async function startMinesPro(bet, minesCount) {
  return api.post('/game/mines-pro/start', { bet, mines_count: minesCount })
}

export async function revealMinesPro(cell) {
  return api.post('/game/mines-pro/reveal', { cell })
}

export async function cashoutMinesPro() {
  return api.post('/game/mines-pro/cashout', {})
}

export async function getMinesProState() {
  return api.get('/game/mines-pro/state')
}

export async function getMinesProInfo() {
  return api.get('/game/mines-pro/info')
}
