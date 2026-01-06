import { httpClient } from './httpClient'

export type AsterNonceType = 'WEB3_LOGIN' | 'CREATE_API_KEY'

export interface AsterWalletConnection {
  walletAddress: string
  agentCode: string
  apiCreated: boolean
  message?: string
}

interface AsterNonceResponsePayload {
  success: boolean
  message?: string
  data?: {
    nonce?: string
  }
}

interface AsterConnectResponsePayload {
  success: boolean
  message?: string
  data?: {
    wallet_address?: string
    agent_code?: string
    api_created?: boolean
  }
}

type EthereumProvider = {
  request: (args: { method: string; params?: unknown[] }) => Promise<any>
  isMetaMask?: boolean
}

function getEthereumProvider(): EthereumProvider | null {
  if (typeof window === 'undefined') return null
  return (window as any).ethereum ?? null
}

function normalizeError(err: unknown, fallback: string): string {
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err.trim()) return err
  return fallback
}

async function requestWalletAddress(): Promise<string> {
  const provider = getEthereumProvider()
  if (!provider) {
    throw new Error('MetaMask not available')
  }
  const accounts = await provider.request({ method: 'eth_requestAccounts' })
  if (!accounts || !Array.isArray(accounts) || accounts.length === 0) {
    throw new Error('Failed to fetch wallet address')
  }
  return String(accounts[0])
}

async function signMessage(walletAddress: string, message: string): Promise<string> {
  const provider = getEthereumProvider()
  if (!provider) {
    throw new Error('MetaMask not available')
  }
  return provider.request({
    method: 'personal_sign',
    params: [message, walletAddress],
  })
}

async function getAsterNonce(
  walletAddress: string,
  nonceType: AsterNonceType
): Promise<string> {
  const response = await httpClient.post<AsterNonceResponsePayload>(
    '/api/aster/nonce',
    {
      wallet_address: walletAddress,
      nonce_type: nonceType,
    }
  )

  if (!response.success || !response.data) {
    throw new Error(response.message || 'Failed to get nonce')
  }
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to get nonce')
  }
  const nonce = response.data.data?.nonce
  if (!nonce) {
    throw new Error('Nonce not returned')
  }
  return nonce
}

export async function connectAsterWallet(): Promise<AsterWalletConnection> {
  const walletAddress = await requestWalletAddress()

  const loginNonce = await getAsterNonce(walletAddress, 'WEB3_LOGIN')
  const loginMessage = `You are signing into Astherus ${loginNonce}`
  const loginSignature = await signMessage(walletAddress, loginMessage)

  const createNonce = await getAsterNonce(walletAddress, 'CREATE_API_KEY')
  const createMessage = `You are signing into Astherus ${createNonce}`
  const createSignature = await signMessage(walletAddress, createMessage)

  const response = await httpClient.post<AsterConnectResponsePayload>(
    '/api/aster/connect',
    {
      wallet_address: walletAddress,
      login_signature: loginSignature,
      login_nonce: loginNonce,
      create_signature: createSignature,
      create_nonce: createNonce,
    }
  )

  if (!response.success || !response.data) {
    throw new Error(response.message || 'Failed to connect Aster wallet')
  }
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to connect Aster wallet')
  }

  return {
    walletAddress: response.data.data?.wallet_address || walletAddress,
    agentCode: response.data.data?.agent_code || '',
    apiCreated: response.data.data?.api_created ?? false,
    message: response.data.message,
  }
}

export function isMetaMaskInstalled(): boolean {
  const provider = getEthereumProvider()
  return Boolean(provider)
}

export async function getCurrentWalletAddress(): Promise<string | null> {
  const provider = getEthereumProvider()
  if (!provider) return null
  try {
    const accounts = await provider.request({ method: 'eth_accounts' })
    if (!accounts || !Array.isArray(accounts) || accounts.length === 0) {
      return null
    }
    return String(accounts[0])
  } catch (err) {
    throw new Error(normalizeError(err, 'Failed to read wallet address'))
  }
}

declare global {
  interface Window {
    ethereum?: EthereumProvider
  }
}
