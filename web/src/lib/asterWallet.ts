/**
 * Aster Wallet 自動生成模式連接庫
 *
 * 流程：
 * 1. 連接 MetaMask 獲取錢包地址
 * 2. 發送主錢包地址到後端 API
 * 3. 後端自動生成 API Wallet 並完成登入/綁定
 */

// ============================================================================
// 類型定義
// ============================================================================

export interface AsterWalletConnection {
  success: boolean;
  walletAddress: string;
  agentCode: string;
  apiCreated: boolean;
  message?: string;
}

interface AsterNonceResponse {
  code: string;
  message?: string;
  msg?: string;
  data: {
    nonce: string;
  };
}

// ============================================================================
// Aster API 交互
// ============================================================================

/**
 * 從 Aster API 獲取 nonce
 * @param walletAddress 錢包地址
 * @param type 類型："WEB3_LOGIN" 或 "CREATE_API_KEY"
 */
async function getAsterNonce(
  walletAddress: string,
  type: 'WEB3_LOGIN' | 'CREATE_API_KEY'
): Promise<string> {
  const baseURL = 'https://www.asterdex.com';
  const endpoint = '/bapi/futures/v1/public/future/web3/get-nonce';

  const response = await fetch(baseURL + endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      sourceAddr: walletAddress,
      network: '56', // BSC Chain ID
      type: type,
    }),
  });

  if (!response.ok) {
    throw new Error(`Failed to get nonce: ${response.statusText}`);
  }

  const data: AsterNonceResponse = await response.json();

  if (data.code !== '000000') {
    const errorMessage = data.message || data.msg || 'Unknown Aster API error';
    throw new Error(`Aster API error: ${errorMessage}`);
  }

  return data.data.nonce;
}

/**
 * 使用 MetaMask 簽名消息
 * @param message 要簽名的消息
 * @param walletAddress 簽名者地址
 */
async function signWithMetaMask(
  message: string,
  walletAddress: string
): Promise<string> {
  // 檢查 MetaMask
  if (!window.ethereum) {
    throw new Error('請安裝 MetaMask 擴展！');
  }

  try {
    // 使用 eth_sign 方法（EIP-191 標準）
    const signature = await window.ethereum.request({
      method: 'personal_sign',
      params: [message, walletAddress],
    });

    return signature;
  } catch (error: any) {
    if (error.code === 4001) {
      throw new Error('用戶拒絕簽名');
    }
    throw error;
  }
}

// ============================================================================
// 主要連接函數
// ============================================================================

/**
 * 連接 Aster 錢包（API Wallet 自動生成模式）
 *
 * 用戶只需提供主錢包地址，後端處理所有簽名與 API Wallet 生成。
 */
export async function connectAsterWallet(
  authToken?: string
): Promise<AsterWalletConnection> {
  try {
    // ========================================
    // 步驟 1: 連接 MetaMask
    // ========================================
    if (!window.ethereum) {
      throw new Error('請安裝 MetaMask 擴展！');
    }

    // 請求連接錢包
    const accounts = await window.ethereum.request({
      method: 'eth_requestAccounts',
    });

    if (!accounts || accounts.length === 0) {
      throw new Error('未能獲取錢包地址');
    }

    const walletAddress = accounts[0];
    console.log('📱 已連接錢包:', walletAddress);

    // ========================================
    // 步驟 2: 發送到後端 API
    // ========================================
    console.log('📡 發送錢包地址到後端...');

    const response = await fetch('/api/aster/connect', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
      },
      body: JSON.stringify({
        wallet_address: walletAddress,
      }),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(
        errorData.message || `後端錯誤: ${response.statusText}`
      );
    }

    const result = await response.json();

    console.log('✅ Aster 錢包連接成功！');

    return {
      success: result.success,
      walletAddress: walletAddress,
      agentCode: result.data?.agent_code || '3E58dc',
      apiCreated: result.data?.api_created ?? false,
      message: result.message,
    };
  } catch (error) {
    console.error('❌ 連接失敗:', error);
    throw error;
  }
}

// ============================================================================
// 工具函數
// ============================================================================

/**
 * 檢查 MetaMask 是否已安裝
 */
export function isMetaMaskInstalled(): boolean {
  return typeof window !== 'undefined' && Boolean(window.ethereum);
}

/**
 * 獲取當前連接的錢包地址（如果有）
 */
export async function getCurrentWalletAddress(): Promise<string | null> {
  if (!window.ethereum) {
    return null;
  }

  try {
    const accounts = await window.ethereum.request({
      method: 'eth_accounts',
    });
    return accounts && accounts.length > 0 ? accounts[0] : null;
  } catch {
    return null;
  }
}

// ============================================================================
// TypeScript 類型擴展
// ============================================================================

declare global {
  interface Window {
    ethereum?: {
      request: (args: { method: string; params?: unknown[] }) => Promise<any>;
      isMetaMask?: boolean;
      on?: (event: string, callback: (...args: any[]) => void) => void;
      removeListener?: (event: string, callback: (...args: any[]) => void) => void;
    };
  }
}
