package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// btcPriceCache is a simple in-memory cache for the live BTC/USD rate.
// It fetches from CoinGecko (no API key required) at most once every 5 minutes.
var btcPriceCache struct {
	mu        sync.RWMutex
	rate      float64
	fetchedAt time.Time
}

const btcPriceTTL = 5 * time.Minute

// GetBtcUsdRate returns the current BTC/USD rate, fetching from CoinGecko
// when the cached value is stale. Returns 0.0 if the price cannot be fetched.
func GetBtcUsdRate() float64 {
	btcPriceCache.mu.RLock()
	if time.Since(btcPriceCache.fetchedAt) < btcPriceTTL && btcPriceCache.rate > 0 {
		r := btcPriceCache.rate
		btcPriceCache.mu.RUnlock()
		return r
	}
	btcPriceCache.mu.RUnlock()

	rate := fetchBtcUsdFromCoinGecko()

	btcPriceCache.mu.Lock()
	if rate > 0 {
		btcPriceCache.rate = rate
		btcPriceCache.fetchedAt = time.Now()
	}
	btcPriceCache.mu.Unlock()

	return btcPriceCache.rate // return last known rate even if fresh fetch failed
}

func fetchBtcUsdFromCoinGecko() float64 {
	const url = "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd"
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url) // #nosec G107 -- URL is a compile-time constant
	if err != nil {
		log.Printf("[price] CoinGecko fetch error: %v", err)
		return 0
	}
	defer resp.Body.Close()

	var data map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("[price] CoinGecko decode error: %v", err)
		return 0
	}

	rate := data["bitcoin"]["usd"]
	if rate <= 0 {
		log.Printf("[price] CoinGecko returned unexpected rate: %v", rate)
		return 0
	}
	log.Printf("[price] BTC/USD refreshed: %.2f", rate)
	return rate
}
