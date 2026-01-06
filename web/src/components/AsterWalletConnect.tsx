/**
 * Aster Wallet 連接組件
 *
 * 提供用戶友好的 UI 來連接 Aster 錢包
 * 完全使用簽名模式，不需要用戶提供私鑰
 */

import React, { useState, useEffect } from 'react';
import {
  connectAsterWallet,
  isMetaMaskInstalled,
  getCurrentWalletAddress,
  type AsterWalletConnection,
} from '../lib/asterWallet';

// ============================================================================
// 組件屬性
// ============================================================================

interface AsterWalletConnectProps {
  /** 連接成功後的回調 */
  onConnected?: (connection: AsterWalletConnection) => void;
  /** 連接失敗後的回調 */
  onError?: (error: Error) => void;
  /** JWT token（受保護 API 需要） */
  authToken?: string;
  /** 自定義樣式類名 */
  className?: string;
  /** 是否顯示詳細信息 */
  showDetails?: boolean;
}

// ============================================================================
// 主組件
// ============================================================================

export function AsterWalletConnect({
  onConnected,
  onError,
  authToken,
  className = '',
  showDetails = true,
}: AsterWalletConnectProps) {
  // ========================================
  // 狀態管理
  // ========================================

  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [connection, setConnection] = useState<AsterWalletConnection | null>(
    null
  );
  const [error, setError] = useState<string>('');
  const [currentStep, setCurrentStep] = useState<string>('');
  const [hasMetaMask, setHasMetaMask] = useState(true);
  const [existingWallet, setExistingWallet] = useState<string | null>(null);

  // ========================================
  // 初始化檢查
  // ========================================

  useEffect(() => {
    // 檢查 MetaMask
    setHasMetaMask(isMetaMaskInstalled());

    // 檢查是否已有連接的錢包
    getCurrentWalletAddress().then((address) => {
      if (address) {
        setExistingWallet(address);
      }
    });
  }, []);

  // ========================================
  // 連接處理
  // ========================================

  const handleConnect = async () => {
    setLoading(true);
    setError('');
    setCurrentStep('準備連接...');

    try {
      // 步驟 1: 連接 MetaMask
      setCurrentStep('連接 MetaMask...');
      await new Promise((resolve) => setTimeout(resolve, 300)); // 視覺延遲

      // 步驟 2: 提交連接請求（後端自動生成 API Wallet）
      setCurrentStep('提交連接請求...');
      await new Promise((resolve) => setTimeout(resolve, 300));

      // 執行完整連接流程
      const result = await connectAsterWallet(authToken);

      setConnection(result);
      setConnected(true);
      setCurrentStep('連接成功！');

      // 回調
      if (onConnected) {
        onConnected(result);
      }

      // 顯示成功訊息
      console.log('✅ Aster 錢包連接成功:', result);
    } catch (err: any) {
      const errorMessage = err.message || '連接失敗';
      setError(errorMessage);
      console.error('❌ 連接失敗:', err);

      // 回調
      if (onError) {
        onError(err);
      }
    } finally {
      setLoading(false);
      setCurrentStep('');
    }
  };

  // ========================================
  // 斷開連接
  // ========================================

  const handleDisconnect = () => {
    setConnected(false);
    setConnection(null);
    setError('');
    setExistingWallet(null);
  };

  // ========================================
  // 渲染：MetaMask 未安裝
  // ========================================

  if (!hasMetaMask) {
    return (
      <div className={`aster-wallet-connect ${className}`}>
        <div className="alert alert-warning">
          <h4>需要安裝 MetaMask</h4>
          <p>請先安裝 MetaMask 瀏覽器擴展來使用 Aster 錢包功能。</p>
          <a
            href="https://metamask.io/download/"
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-primary"
          >
            下載 MetaMask
          </a>
        </div>
      </div>
    );
  }

  // ========================================
  // 渲染：已連接狀態
  // ========================================

  if (connected && connection) {
    return (
      <div className={`aster-wallet-connect connected ${className}`}>
        <div className="connection-success">
          <div className="success-header">
            <span className="success-icon">✅</span>
            <h3>Aster 錢包已連接</h3>
          </div>

          {showDetails && (
            <div className="connection-details">
              <div className="detail-item">
                <span className="detail-label">錢包地址:</span>
                <span className="detail-value wallet-address">
                  {connection.walletAddress.slice(0, 6)}...
                  {connection.walletAddress.slice(-4)}
                </span>
                <button
                  className="btn-copy"
                  onClick={() => {
                    navigator.clipboard.writeText(connection.walletAddress);
                    alert('已複製錢包地址');
                  }}
                  title="複製完整地址"
                >
                  📋
                </button>
              </div>

              <div className="detail-item">
                <span className="detail-label">Referral Code:</span>
                <span className="detail-value agent-code">
                  {connection.agentCode}
                </span>
              </div>

              <div className="detail-item">
                <span className="detail-label">API Wallet 狀態:</span>
                <span className="detail-value">
                  {connection.apiCreated ? '✅ 已就緒' : '⏳ 建立中...'}
                </span>
              </div>

              {connection.message && (
                <div className="detail-item">
                  <span className="detail-label">訊息:</span>
                  <span className="detail-value">{connection.message}</span>
                </div>
              )}
            </div>
          )}

          <button className="btn btn-secondary" onClick={handleDisconnect}>
            斷開連接
          </button>
        </div>
      </div>
    );
  }

  // ========================================
  // 渲染：未連接狀態
  // ========================================

  return (
    <div className={`aster-wallet-connect ${className}`}>
      <div className="connect-prompt">
        <h3>連接 Aster 錢包</h3>
        <p className="description">
          使用 MetaMask 簽名授權連接到 Aster DEX。
          <br />
          您的私鑰永遠不會離開您的瀏覽器。
        </p>

        {existingWallet && (
          <div className="existing-wallet-notice">
            <span>🔗 檢測到錢包: </span>
            <span className="wallet-address">
              {existingWallet.slice(0, 6)}...{existingWallet.slice(-4)}
            </span>
          </div>
        )}

        <button
          onClick={handleConnect}
          disabled={loading}
          className={`btn btn-primary btn-connect ${loading ? 'loading' : ''}`}
        >
          {loading ? (
            <>
              <span className="spinner">⏳</span>
              <span>{currentStep || '連接中...'}</span>
            </>
          ) : (
            <>
              <span>🦊</span>
              <span>連接 MetaMask</span>
            </>
          )}
        </button>

        {showDetails && (
          <div className="connection-steps">
            <p className="steps-title">連接步驟：</p>
            <ol>
              <li>連接 MetaMask 錢包</li>
              <li>後端自動生成 API Wallet</li>
              <li>自動綁定 Referral Code</li>
            </ol>
          </div>
        )}

        {error && (
          <div className="alert alert-error">
            <strong>❌ 連接失敗</strong>
            <p>{error}</p>
            {error.includes('拒絕') && (
              <p className="error-hint">
                提示: 請在 MetaMask 中批准簽名請求
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ============================================================================
// 默認導出
// ============================================================================

export default AsterWalletConnect;
