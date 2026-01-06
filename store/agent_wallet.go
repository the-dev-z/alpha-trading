package store

import (
	"nofx/crypto"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AgentWalletStore handles Hyperliquid agent wallet storage.
type AgentWalletStore struct {
	db *gorm.DB
}

// AgentWallet represents a Hyperliquid agent wallet record.
type AgentWallet struct {
	ID                     uint                  `gorm:"primaryKey" json:"id"`
	UserID                 string                `gorm:"column:user_id;not null;default:default;uniqueIndex:idx_agent_wallet_user_main" json:"user_id"`
	MainWallet             string                `gorm:"column:main_wallet;not null;default:'';uniqueIndex:idx_agent_wallet_user_main" json:"main_wallet"`
	AgentAddress           string                `gorm:"column:agent_address;not null;default:'';uniqueIndex" json:"agent_address"`
	EncryptedPrivateKey    crypto.EncryptedString `gorm:"column:encrypted_private_key;not null;default:''" json:"-"`
	AuthorizationSignature string                `gorm:"column:authorization_signature;default:''" json:"-"`
	Status                 string                `gorm:"column:status;default:'INIT'" json:"status"`
	HyperliquidChain       string                `gorm:"column:hyperliquid_chain;default:'Mainnet'" json:"hyperliquid_chain"`
	BuilderAddress         string                `gorm:"column:builder_address;default:''" json:"builder_address"`
	BuilderFeeAuthorized   bool                  `gorm:"column:builder_fee_authorized;default:false" json:"builder_fee_authorized"`
	BuilderFeeMaxRate      int                   `gorm:"column:builder_fee_max_rate;default:0" json:"builder_fee_max_rate"`
	BuilderFeeAuthorizedAt *time.Time            `gorm:"column:builder_fee_authorized_at" json:"builder_fee_authorized_at,omitempty"`
	CreatedAt              time.Time             `json:"created_at"`
	UpdatedAt              time.Time             `json:"updated_at"`
}

// NewAgentWalletStore creates a new AgentWalletStore.
func NewAgentWalletStore(db *gorm.DB) *AgentWalletStore {
	return &AgentWalletStore{db: db}
}

func (s *AgentWalletStore) initTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'agent_wallets'`).Scan(&tableExists)
		if tableExists > 0 {
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT 'default'`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS main_wallet TEXT NOT NULL DEFAULT ''`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS agent_address TEXT NOT NULL DEFAULT ''`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS encrypted_private_key TEXT NOT NULL DEFAULT ''`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS authorization_signature TEXT DEFAULT ''`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'INIT'`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS hyperliquid_chain TEXT DEFAULT 'Mainnet'`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS builder_address TEXT DEFAULT ''`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS builder_fee_authorized BOOLEAN DEFAULT FALSE`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS builder_fee_max_rate INTEGER DEFAULT 0`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS builder_fee_authorized_at TIMESTAMP`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
			s.db.Exec(`ALTER TABLE agent_wallets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
			s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_wallet_user_main ON agent_wallets(user_id, main_wallet)`)
			s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_wallets_agent_address ON agent_wallets(agent_address)`)
			return nil
		}
	}

	return s.db.AutoMigrate(&AgentWallet{})
}

// Create inserts a new agent wallet record.
func (s *AgentWalletStore) Create(userID, mainWallet, agentAddress, privateKeyHex, chain string) (*AgentWallet, error) {
	wallet := &AgentWallet{
		UserID:              strings.TrimSpace(userID),
		MainWallet:          normalizeWallet(mainWallet),
		AgentAddress:        strings.ToLower(strings.TrimSpace(agentAddress)),
		EncryptedPrivateKey: crypto.EncryptedString(strings.TrimSpace(privateKeyHex)),
		Status:              "INIT",
		HyperliquidChain:    strings.TrimSpace(chain),
	}
	if err := s.db.Create(wallet).Error; err != nil {
		return nil, err
	}
	return wallet, nil
}

// GetByMainWallet fetches an agent wallet by user and main wallet address.
func (s *AgentWalletStore) GetByMainWallet(userID, mainWallet string) (*AgentWallet, error) {
	var wallet AgentWallet
	err := s.db.Where("user_id = ? AND main_wallet = ?", strings.TrimSpace(userID), normalizeWallet(mainWallet)).First(&wallet).Error
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// GetActiveByMainWallet fetches an active agent wallet by user and main wallet.
func (s *AgentWalletStore) GetActiveByMainWallet(userID, mainWallet string) (*AgentWallet, error) {
	var wallet AgentWallet
	err := s.db.Where("user_id = ? AND main_wallet = ? AND status = ?", strings.TrimSpace(userID), normalizeWallet(mainWallet), "ACTIVE").First(&wallet).Error
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// DeleteByMainWallet deletes an agent wallet by user and main wallet.
func (s *AgentWalletStore) DeleteByMainWallet(userID, mainWallet string) error {
	return s.db.Where("user_id = ? AND main_wallet = ?", strings.TrimSpace(userID), normalizeWallet(mainWallet)).Delete(&AgentWallet{}).Error
}

// UpdateAuthorization updates authorization signature and activates the wallet.
func (s *AgentWalletStore) UpdateAuthorization(userID, mainWallet, signature string) error {
	updates := map[string]interface{}{
		"authorization_signature": strings.TrimSpace(signature),
		"status":                  "ACTIVE",
	}
	return s.db.Model(&AgentWallet{}).
		Where("user_id = ? AND main_wallet = ?", strings.TrimSpace(userID), normalizeWallet(mainWallet)).
		Updates(updates).Error
}

// UpdateBuilderFee updates builder fee authorization info.
func (s *AgentWalletStore) UpdateBuilderFee(userID, mainWallet, builderAddress string, maxFeeRate int, authorizedAt time.Time) error {
	updates := map[string]interface{}{
		"builder_fee_authorized":    true,
		"builder_fee_max_rate":      maxFeeRate,
		"builder_fee_authorized_at": authorizedAt,
		"builder_address":           strings.ToLower(strings.TrimSpace(builderAddress)),
	}
	return s.db.Model(&AgentWallet{}).
		Where("user_id = ? AND main_wallet = ?", strings.TrimSpace(userID), normalizeWallet(mainWallet)).
		Updates(updates).Error
}

func normalizeWallet(wallet string) string {
	trimmed := strings.TrimSpace(wallet)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}
