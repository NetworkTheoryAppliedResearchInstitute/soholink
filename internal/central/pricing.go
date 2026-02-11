package central

import (
	"time"
)

// TransactionFee represents the revenue split for a single transaction.
// Central SOHO takes 1% of the net amount (after payment processor fees),
// and the producer (thin client operator) receives the remaining 99%.
type TransactionFee struct {
	TransactionID  string
	TotalAmount    int64  // Total transaction value in cents
	CentralFee     int64  // 1% fee in cents
	ProducerPayout int64  // 99% to producer in cents
	UserPaid       int64  // What user originally paid
	ProcessorFee   int64  // Stripe/payment processor fee
	NetAmount      int64  // After processor fee
	Currency       string
	CreatedAt      time.Time
}

// CalculateFees computes the revenue distribution for a transaction.
//
// Example: User pays $10.00 = 1000 cents
//
//	Processor (Stripe) takes 2.9% + 30¢ = 59 cents
//	Net: 1000 - 59 = 941 cents
//	Central SOHO: 941 * 0.01 = 9 cents (truncated)
//	Producer: 941 - 9 = 932 cents
func CalculateFees(userPayment int64, processorFeePercent float64) TransactionFee {
	processorFee := int64(float64(userPayment) * processorFeePercent)
	netAmount := userPayment - processorFee

	centralFee := netAmount / 100 // 1% of net (integer truncation)
	producerPayout := netAmount - centralFee

	return TransactionFee{
		TotalAmount:    userPayment,
		CentralFee:     centralFee,
		ProducerPayout: producerPayout,
		UserPaid:       userPayment,
		ProcessorFee:   processorFee,
		NetAmount:      netAmount,
		Currency:       "USD",
		CreatedAt:      time.Now(),
	}
}

// CalculateFeesWithFixed computes the revenue distribution when the processor
// charges a percentage plus a fixed fee (e.g., Stripe's 2.9% + $0.30).
func CalculateFeesWithFixed(userPayment int64, percentFee float64, fixedFeeCents int64) TransactionFee {
	processorFee := int64(float64(userPayment)*percentFee) + fixedFeeCents
	if processorFee > userPayment {
		processorFee = userPayment
	}
	netAmount := userPayment - processorFee

	centralFee := netAmount / 100 // 1%
	producerPayout := netAmount - centralFee

	return TransactionFee{
		TotalAmount:    userPayment,
		CentralFee:     centralFee,
		ProducerPayout: producerPayout,
		UserPaid:       userPayment,
		ProcessorFee:   processorFee,
		NetAmount:      netAmount,
		Currency:       "USD",
		CreatedAt:      time.Now(),
	}
}
