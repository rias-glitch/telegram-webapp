import { useState, useCallback, useRef, useEffect } from 'react'
import { getToken } from '../api/client'

export function useWebSocket(gameType = 'rps') {
  const [status, setStatus] = useState('disconnected') // disconnected, connecting, waiting, matched, playing
  const [opponent, setOpponent] = useState(null)
  const [roomId, setRoomId] = useState(null)
  const [gameState, setGameState] = useState(null)
  const [result, setResult] = useState(null)

  const wsRef = useRef(null)
  const handlersRef = useRef({})

  const connect = useCallback((betAmount) => {
    const token = getToken()
    if (!token) {
      console.error('No auth token')
      return
    }

    let wsUrl
    const wsBase = import.meta.env.VITE_WS_URL
    if (wsBase) {
      wsUrl = `${wsBase}/ws?token=${token}&bet=${betAmount}&game=${gameType}`
    } else {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      wsUrl = `${protocol}//${window.location.host}/ws?token=${token}&bet=${betAmount}&game=${gameType}`
    }

    setStatus('connecting')
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('waiting')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        handleMessage(msg)
      } catch (err) {
        console.error('Failed to parse message:', err)
      }
    }

    ws.onclose = () => {
      setStatus('disconnected')
      setOpponent(null)
      setRoomId(null)
      wsRef.current = null
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
    }
  }, [gameType])

  const handleMessage = useCallback((msg) => {
    switch (msg.type) {
      case 'matched':
        setStatus('matched')
        setOpponent(msg.opponent)
        setRoomId(msg.room_id)
        break

      case 'start':
        setStatus('playing')
        setGameState(msg.state)
        break

      case 'state':
        setGameState(msg.state)
        break

      case 'result':
        setResult(msg)
        setStatus('disconnected')
        break

      case 'opponent_left':
        setResult({ type: 'opponent_left', message: 'Opponent left the game' })
        setStatus('disconnected')
        break

      default:
        if (handlersRef.current[msg.type]) {
          handlersRef.current[msg.type](msg)
        }
    }
  }, [])

  const send = useCallback((data) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setStatus('disconnected')
    setOpponent(null)
    setRoomId(null)
    setGameState(null)
    setResult(null)
  }, [])

  const onMessage = useCallback((type, handler) => {
    handlersRef.current[type] = handler
  }, [])

  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [])

  return {
    status,
    opponent,
    roomId,
    gameState,
    result,
    connect,
    send,
    disconnect,
    onMessage,
  }
}
