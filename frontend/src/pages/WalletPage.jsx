import { useState, useEffect, useCallback } from 'react'
import { useTonConnectUI, useTonWallet } from '@tonconnect/ui-react'
import { Card, CardTitle, Button, Input } from '../components/ui'
import * as tonApi from '../api/ton'

const STATUS_COLORS = {
  pending: 'text-yellow-400',
  confirmed: 'text-success',
  completed: 'text-success',
  failed: 'text-danger',
  cancelled: 'text-white/40',
}

export function WalletPage({ user }) {
  const [tonConnectUI] = useTonConnectUI()
  const tonWallet = useTonWallet()

  const [wallet, setWallet] = useState(null)
  const [config, setConfig] = useState(null)
  const [deposits, setDeposits] = useState([])
  const [withdrawals, setWithdrawals] = useState([])
  const [depositInfo, setDepositInfo] = useState(null)
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('deposit')
  const [connecting, setConnecting] = useState(false)

  // Withdraw form
  const [withdrawAmount, setWithdrawAmount] = useState(10)
  const [withdrawEstimate, setWithdrawEstimate] = useState(null)
  const [withdrawing, setWithdrawing] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [configRes, walletRes, depositsRes, withdrawalsRes] =
        await Promise.all([
          tonApi.getTonConfig(),
          tonApi.getWallet(),
          tonApi.getDeposits(),
          tonApi.getWithdrawals(),
        ])
      setConfig(configRes)
      setWallet(walletRes.wallet)
      setDeposits(depositsRes.deposits || [])
      setWithdrawals(withdrawalsRes.withdrawals || [])
    } catch (err) {
      console.error('Failed to fetch wallet data:', err)
    }

    // Always try to load deposit info (in separate try/catch)
    try {
      console.log('Loading deposit info...')
      const info = await tonApi.getDepositInfo()
      console.log('Deposit info loaded:', info)
      setDepositInfo(info)
    } catch (err) {
      console.error('Failed to fetch deposit info:', err)
      // Set error object to stop loading indicator
      setDepositInfo({ error: err.message || 'Failed to load deposit info' })
    }

    setLoading(false)
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // Sync TON Connect wallet with backend
  useEffect(() => {
    const syncWallet = async () => {
      if (tonWallet && !wallet && !connecting) {
        setConnecting(true)
        try {
          // Get proof from TON Connect
          const proof = tonConnectUI.wallet?.connectItems?.tonProof

          await tonApi.connectWallet(
            {
              address: tonWallet.account.address,
              chain: tonWallet.account.chain,
              publicKey: tonWallet.account.publicKey,
            },
            proof || {
              timestamp: Date.now(),
              domain: { value: window.location.host },
            },
          )
          await fetchData()
        } catch (err) {
          console.error('Failed to sync wallet:', err)
        } finally {
          setConnecting(false)
        }
      }
    }
    syncWallet()
  }, [tonWallet, wallet, tonConnectUI, connecting, fetchData])

  // Calculate withdraw estimate when amount changes
  useEffect(() => {
    if (withdrawAmount >= 10) {
      tonApi
        .getWithdrawEstimate(withdrawAmount)
        .then(setWithdrawEstimate)
        .catch(() => setWithdrawEstimate(null))
    } else {
      setWithdrawEstimate(null)
    }
  }, [withdrawAmount])

  const handleConnect = async () => {
    try {
      await tonConnectUI.openModal()
    } catch (err) {
      console.error('Failed to open TON Connect:', err)
    }
  }

  const handleDisconnect = async () => {
    try {
      await tonConnectUI.disconnect()
      await tonApi.disconnectWallet()
      setWallet(null)
      await fetchData()
    } catch (err) {
      console.error('Failed to disconnect:', err)
    }
  }

  const handleWithdraw = async () => {
    if (!withdrawEstimate || withdrawing) return

    try {
      setWithdrawing(true)
      await tonApi.requestWithdrawal(withdrawAmount)
      await fetchData()
      setWithdrawAmount(10)
      alert('Withdrawal request created!')
    } catch (err) {
      alert(err.message)
    } finally {
      setWithdrawing(false)
    }
  }

  const handleCancelWithdrawal = async id => {
    try {
      await tonApi.cancelWithdrawal(id)
      await fetchData()
    } catch (err) {
      alert(err.message)
    }
  }

  const handleQuickDeposit = async tonAmount => {
    if (!tonConnectUI || !depositInfo) {
      alert('Please wait for wallet to connect')
      return
    }

    try {
      // Convert TON to nanoTON (1 TON = 1,000,000,000 nanoTON)
      const nanoTON = Math.floor(tonAmount * 1_000_000_000).toString()

      // Create transaction (simple transfer without payload)
      const transaction = {
        validUntil: Math.floor(Date.now() / 1000) + 600, // 10 minutes
        messages: [
          {
            address: depositInfo.platform_address,
            amount: nanoTON,
          },
        ],
      }

      console.log('Sending transaction:', transaction)

      // Send transaction (this will open wallet for confirmation)
      const result = await tonConnectUI.sendTransaction(transaction)

      console.log('Transaction result:', result)

      if (result) {
        alert(
          `Transaction sent! Your coins will be credited after confirmation.`,
        )
        // Refresh data after a delay to check for deposit
        setTimeout(() => fetchData(), 5000)
      }
    } catch (err) {
      console.error('Quick deposit failed:', err)
      if (
        err.message?.includes('user rejects') ||
        err.message?.includes('User rejected')
      ) {
        alert('Transaction cancelled')
      } else {
        alert('Failed to send transaction: ' + err.message)
      }
    }
  }

  const formatDate = dateStr => {
    return new Date(dateStr).toLocaleDateString('ru-RU', {
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const shortenAddress = addr => {
    if (!addr) return ''
    return `${addr.slice(0, 6)}...${addr.slice(-4)}`
  }

  if (loading) {
    return (
      <div className='flex items-center justify-center py-12'>
        <div className='text-4xl animate-pulse'>💎</div>
      </div>
    )
  }

  const isConnected = wallet || tonWallet

  return (
    <div className='space-y-4 animate-fadeIn'>
      <h1 className='text-2xl font-bold'>Кошелёк</h1>

      {/* Balance card */}
      <Card className='bg-gradient-to-r from-blue-500/20 to-cyan-500/20 border-blue-500/30'>
        <div className='text-center'>
          <div className='text-white/60 text-sm mb-1'>Ваши Coins</div>
          <div className='text-4xl font-bold flex items-center justify-center gap-2'>
            <span>🪙</span>
            <span>{user?.coins?.toLocaleString() || 0}</span>
          </div>
          {config && (
            <div className='text-white/40 text-sm mt-2'>
              1 TON = {config.coins_per_ton} coins
            </div>
          )}
        </div>
      </Card>

      {/* Wallet status */}
      {isConnected ? (
        <Card>
          <div className='flex items-center justify-between'>
            <div>
              <div className='text-white/60 text-sm'>Подключённый кошелёк</div>
              <div className='font-mono text-sm'>
                {shortenAddress(wallet?.address || tonWallet?.account?.address)}
              </div>
            </div>
            <div className='flex items-center gap-2'>
              <span className='text-success text-sm'>Подключено</span>
              <Button size='sm' variant='secondary' onClick={handleDisconnect}>
                Отключить
              </Button>
            </div>
          </div>
        </Card>
      ) : (
        <Card className='text-center py-6'>
          <div className='text-4xl mb-3'>🔗</div>
          <p className='text-white/60 mb-4'>
            Подключи свой TON кошелёк для покупки и вывода coins
          </p>
          <Button onClick={handleConnect} className='mx-auto'>
            {connecting ? 'Подключение...' : 'Подключить кошелёк'}
          </Button>
          <p className='text-xs text-white/40 mt-3'>
            Tonkeeper, Tonhub, OpenMask, MyTonWallet
          </p>
        </Card>
      )}

      {/* Tabs */}
      {isConnected && (
        <>
          <div className='flex gap-2'>
            {[
              { key: 'deposit', label: 'Пополнение' },
              { key: 'withdraw', label: 'Вывод' },
              { key: 'history', label: 'История' },
            ].map(tab => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`flex-1 py-2 rounded-xl font-medium transition-colors ${
                  activeTab === tab.key
                    ? 'bg-primary text-white'
                    : 'bg-white/10 text-white/60 hover:bg-white/20'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          {/* Deposit tab */}
          {activeTab === 'deposit' && !depositInfo && (
            <Card className='text-center py-8'>
              <div className='text-4xl mb-2'>⏳</div>
              <p className='text-white/60'>Загрузка информации пополнения...</p>
            </Card>
          )}
          {activeTab === 'deposit' &&
            depositInfo &&
            (depositInfo.error || !depositInfo.platform_address) && (
              <Card className='text-center py-8'>
                <div className='text-4xl mb-2'>⚠️</div>
                <p className='text-danger mb-2'>
                  Ошибка загрузки инфо пополнения
                </p>
                <p className='text-white/60 text-sm'>
                  {depositInfo.error || 'Platform wallet not configured'}
                </p>
                <p className='text-white/40 text-xs mt-4'>
                  Свяжитесь с поддержкой или проверьте конфигурацию сервера
                </p>
                <button
                  onClick={() => fetchData()}
                  className='mt-4 px-4 py-2 bg-primary rounded-lg hover:bg-primary/80 transition-colors'
                >
                  Повторить
                </button>
              </Card>
            )}
          {activeTab === 'deposit' &&
            depositInfo &&
            !depositInfo.error &&
            depositInfo.platform_address && (
              <Card>
                <CardTitle className='mb-4'>Купить Coins</CardTitle>
                <div className='space-y-4'>
                  {/* Quick deposit offers */}
                  <div className='space-y-2'>
                    <label className='text-sm text-white/60'>
                      Быстрое пополнение
                    </label>
                    <div className='grid grid-cols-2 gap-2'>
                      {[
                        { ton: 1, coins: 10 },
                        { ton: 5, coins: 50 },
                        { ton: 10, coins: 100 },
                        { ton: 50, coins: 500 },
                      ].map(offer => (
                        <button
                          key={offer.ton}
                          onClick={() => handleQuickDeposit(offer.ton)}
                          className='bg-gradient-to-r from-primary/20 to-cyan-500/20 border border-primary/50 rounded-xl p-4 hover:from-primary/30 hover:to-cyan-500/30 transition-all hover:scale-105 active:scale-95'
                        >
                          <div className='text-lg font-bold'>
                            {offer.ton} TON
                          </div>
                          <div className='text-success text-sm'>
                            +{offer.coins} coins
                          </div>
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className='relative'>
                    <div className='absolute inset-0 flex items-center'>
                      <div className='w-full border-t border-white/10'></div>
                    </div>
                    <div className='relative flex justify-center text-xs'>
                      <span className='bg-dark px-2 text-white/40'>
                        или пополните вручную
                      </span>
                    </div>
                  </div>

                  <div className='bg-white/5 rounded-xl p-4 text-center'>
                    <div className='text-white/60 text-sm mb-2'>
                      Отправьте TON на этот адрес:
                    </div>
                    <div className='font-mono text-sm break-all bg-dark rounded-lg p-3 mb-2'>
                      {depositInfo.platform_address}
                    </div>
                    <Button
                      size='sm'
                      variant='secondary'
                      onClick={() =>
                        navigator.clipboard.writeText(
                          depositInfo.platform_address,
                        )
                      }
                    >
                      Скопировать адрес
                    </Button>
                  </div>

                  <div className='bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-3'>
                    <div className='flex items-start gap-2'>
                      <span className='text-yellow-400'>!</span>
                      <div className='text-sm'>
                        <p className='text-yellow-400 font-medium'>Важно!</p>
                        <p className='text-white/60'>
                          Используйте этот memo в комментарии:{' '}
                          <span className='font-mono text-white'>
                            {depositInfo.memo}
                          </span>
                        </p>
                      </div>
                    </div>
                  </div>

                  <div className='grid grid-cols-2 gap-3 text-sm'>
                    <div className='bg-white/5 rounded-lg p-3'>
                      <div className='text-white/60'>Минимальный депозит</div>
                      <div className='font-bold'>
                        {depositInfo.min_amount_ton} TON
                      </div>
                    </div>
                    <div className='bg-white/5 rounded-lg p-3'>
                      <div className='text-white/60'>Курс</div>
                      <div className='font-bold'>
                        1 TON = {depositInfo.exchange_rate} coins
                      </div>
                    </div>
                  </div>
                </div>
              </Card>
            )}

          {/* Withdraw tab */}
          {activeTab === 'withdraw' && (
            <Card>
              <CardTitle className='mb-4'>Вывести TON</CardTitle>
              <div className='space-y-4'>
                <div className='space-y-2'>
                  <label className='text-sm text-white/60'>
                    Количество (coins)
                  </label>
                  <Input
                    type='number'
                    value={withdrawAmount}
                    onChange={e =>
                      setWithdrawAmount(
                        Math.max(1, parseInt(e.target.value) || 0),
                      )
                    }
                    min={10}
                    max={user?.coins || 0}
                  />
                  <div className='flex gap-2'>
                    {[10, 50, 100, 500].map(preset => (
                      <button
                        key={preset}
                        onClick={() => setWithdrawAmount(preset)}
                        className={`flex-1 py-1 rounded-lg text-sm transition-colors ${
                          withdrawAmount === preset
                            ? 'bg-primary text-white'
                            : 'bg-white/10 text-white/60 hover:bg-white/20'
                        }`}
                      >
                        {preset}
                      </button>
                    ))}
                  </div>
                </div>

                {withdrawEstimate && (
                  <div className='bg-white/5 rounded-xl p-4 space-y-2'>
                    <div className='flex justify-between text-sm'>
                      <span className='text-white/60'>Количество</span>
                      <span>{withdrawEstimate.coins_amount} coins</span>
                    </div>
                    <div className='flex justify-between text-sm'>
                      <span className='text-white/60'>
                        Комиссия ({withdrawEstimate.fee_ton} TON)
                      </span>
                      <span className='text-danger'>
                        -{withdrawEstimate.fee_coins} coins
                      </span>
                    </div>
                    <div className='border-t border-white/10 pt-2 flex justify-between'>
                      <span className='text-white/60'>Вы получите</span>
                      <span className='font-bold text-success'>
                        {withdrawEstimate.ton_amount} TON
                      </span>
                    </div>
                  </div>
                )}

                <Button
                  onClick={handleWithdraw}
                  disabled={
                    !withdrawEstimate ||
                    withdrawing ||
                    withdrawAmount > (user?.coins || 0)
                  }
                  className='w-full'
                >
                  {withdrawing ? 'Обработка...' : 'Получить вывод средств'}
                </Button>

                {config && (
                  <div className='text-xs text-white/40 text-center'>
                    Min: {config.min_withdraw_coins} coins | Max/day:{' '}
                    {config.max_withdraw_coins_per_day} coins
                  </div>
                )}
              </div>
            </Card>
          )}

          {/* History tab */}
          {activeTab === 'history' && (
            <div className='space-y-3'>
              {deposits.length === 0 && withdrawals.length === 0 ? (
                <Card className='text-center py-8'>
                  <div className='text-4xl mb-2'>📋</div>
                  <p className='text-white/60'>Ваших транзакций еще нет</p>
                </Card>
              ) : (
                <>
                  {/* Pending withdrawals */}
                  {withdrawals
                    .filter(w => w.status === 'pending')
                    .map(w => (
                      <Card key={`w-${w.id}`} className='border-yellow-500/30'>
                        <div className='flex items-center justify-between'>
                          <div>
                            <div className='flex items-center gap-2'>
                              <span>📤</span>
                              <span className='font-medium'>Вывод</span>
                              <span className='text-yellow-400 text-xs'>
                                В ожидании
                              </span>
                            </div>
                            <div className='text-white/60 text-sm'>
                              {w.coins_amount} coins
                            </div>
                          </div>
                          <Button
                            size='sm'
                            variant='secondary'
                            onClick={() => handleCancelWithdrawal(w.id)}
                          >
                            Отменить
                          </Button>
                        </div>
                      </Card>
                    ))}

                  {/* Combined history */}
                  {[
                    ...deposits.map(d => ({ ...d, type: 'deposit' })),
                    ...withdrawals
                      .filter(w => w.status !== 'pending')
                      .map(w => ({ ...w, type: 'withdrawal' })),
                  ]
                    .sort(
                      (a, b) => new Date(b.created_at) - new Date(a.created_at),
                    )
                    .slice(0, 20)
                    .map(tx => (
                      <Card key={`${tx.type}-${tx.id}`}>
                        <div className='flex items-center justify-between'>
                          <div>
                            <div className='flex items-center gap-2'>
                              <span>{tx.type === 'deposit' ? '📥' : '📤'}</span>
                              <span className='font-medium'>
                                {tx.type === 'deposit' ? 'Пополнение' : 'Вывод'}
                              </span>
                            </div>
                            <div className='text-white/40 text-xs'>
                              {formatDate(tx.created_at)}
                            </div>
                          </div>
                          <div className='text-right'>
                            <div
                              className={`font-bold ${tx.type === 'deposit' ? 'text-success' : 'text-white'}`}
                            >
                              {tx.type === 'deposit' ? '+' : '-'}
                              {tx.coins_credited || tx.coins_amount} coins
                            </div>
                            <div
                              className={`text-xs ${STATUS_COLORS[tx.status]}`}
                            >
                              {tx.status}
                            </div>
                          </div>
                        </div>
                      </Card>
                    ))}
                </>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
