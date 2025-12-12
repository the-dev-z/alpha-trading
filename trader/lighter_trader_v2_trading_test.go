package trader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLighterTraderV2_toLighterBaseAmount(t *testing.T) {
	t.Run("rejects non-positive quantity", func(t *testing.T) {
		_, err := toLighterBaseAmount(0)
		assert.Error(t, err)

		_, err = toLighterBaseAmount(-1)
		assert.Error(t, err)
	})

	t.Run("smallest representable", func(t *testing.T) {
		amt, err := toLighterBaseAmount(0.00000001) // 1e-8 -> 1 base tick
		assert.NoError(t, err)
		assert.Equal(t, int64(1), amt)
	})

	t.Run("one unit", func(t *testing.T) {
		amt, err := toLighterBaseAmount(1)
		assert.NoError(t, err)
		assert.Equal(t, int64(100000000), amt)
	})
}

func TestLighterTraderV2_toLighterPriceTicks(t *testing.T) {
	t.Run("rejects non-positive price", func(t *testing.T) {
		_, err := toLighterPriceTicks(0)
		assert.Error(t, err)
	})

	t.Run("rounding variants", func(t *testing.T) {
		ceil, err := toLighterPriceTicksCeil(1.234)
		assert.NoError(t, err)
		assert.Equal(t, uint32(124), ceil)

		floor, err := toLighterPriceTicksFloor(1.239)
		assert.NoError(t, err)
		assert.Equal(t, uint32(123), floor)

		round, err := toLighterPriceTicks(1.235)
		assert.NoError(t, err)
		assert.Equal(t, uint32(124), round)
	})
}

func TestLighterTraderV2_leverageToInitialMarginFraction(t *testing.T) {
	t.Run("rejects non-positive leverage", func(t *testing.T) {
		_, err := leverageToInitialMarginFraction(0)
		assert.Error(t, err)
	})

	t.Run("converts leverage to margin fraction", func(t *testing.T) {
		f, err := leverageToInitialMarginFraction(10)
		assert.NoError(t, err)
		assert.Equal(t, uint16(1000), f) // 10000 / 10
	})
}

func TestLighterTraderV2_getSDKMarketIndex(t *testing.T) {
	tr := &LighterTraderV2{
		marketIndexMap: map[string]int16{
			"BTC-PERP": 0,
			"BTC-SPOT": 2048,
		},
	}

	idx, err := tr.getSDKMarketIndex("BTC-PERP")
	assert.NoError(t, err)
	assert.Equal(t, int16(0), idx)

	_, err = tr.getSDKMarketIndex("BTC-SPOT")
	assert.Error(t, err)
}
