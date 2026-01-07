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
