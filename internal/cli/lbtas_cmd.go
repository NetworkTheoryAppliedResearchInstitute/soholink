package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/lbtas"
)

var lbtasCmd = &cobra.Command{
	Use:   "lbtas",
	Short: "LBTAS reputation management",
}

var lbtasProfileCmd = &cobra.Command{
	Use:   "profile <did>",
	Short: "Show LBTAS profile for a DID",
	Args:  cobra.ExactArgs(1),
	RunE:  runLBTASProfile,
}

var lbtasRateUserCmd = &cobra.Command{
	Use:   "rate-user",
	Short: "Rate a user (provider action)",
	RunE:  runLBTASRateUser,
}

var lbtasRateProviderCmd = &cobra.Command{
	Use:   "rate-provider",
	Short: "Rate a provider (user action)",
	RunE:  runLBTASRateProvider,
}

var (
	rateTransactionID string
	rateScore         int
	rateFeedback      string
	rateCategory      string
)

func init() {
	rootCmd.AddCommand(lbtasCmd)
	lbtasCmd.AddCommand(lbtasProfileCmd)
	lbtasCmd.AddCommand(lbtasRateUserCmd)
	lbtasCmd.AddCommand(lbtasRateProviderCmd)

	lbtasRateUserCmd.Flags().StringVar(&rateTransactionID, "transaction", "", "transaction ID")
	lbtasRateUserCmd.Flags().IntVar(&rateScore, "score", 3, "rating score (0-5)")
	lbtasRateUserCmd.Flags().StringVar(&rateFeedback, "feedback", "", "optional feedback")
	lbtasRateUserCmd.Flags().StringVar(&rateCategory, "category", "execution_quality", "rating category")

	lbtasRateProviderCmd.Flags().StringVar(&rateTransactionID, "transaction", "", "transaction ID")
	lbtasRateProviderCmd.Flags().IntVar(&rateScore, "score", 3, "rating score (0-5)")
	lbtasRateProviderCmd.Flags().StringVar(&rateFeedback, "feedback", "", "optional feedback")
	lbtasRateProviderCmd.Flags().StringVar(&rateCategory, "category", "payment_reliability", "rating category")
}

func runLBTASProfile(cmd *cobra.Command, args []string) error {
	did := args[0]

	s, _, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	ctx := context.Background()
	score, err := s.GetLBTASScore(ctx, did)
	if err != nil {
		return fmt.Errorf("failed to get score: %w", err)
	}

	if score == nil {
		fmt.Printf("No LBTAS profile found for %s\n", did)
		fmt.Println("New users start with a default score of 50/100.")
		return nil
	}

	// Display DID (truncated for readability)
	displayDID := did
	if len(displayDID) > 40 {
		displayDID = displayDID[:40] + "..."
	}

	fmt.Printf("LBTAS Profile: %s\n", displayDID)
	fmt.Println()
	fmt.Printf("Overall Score: %d/100\n", score.OverallScore)
	fmt.Println()

	// Category breakdown table
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Category", "Score")
	table.Append("Payment Reliability", fmt.Sprintf("%.1f/5", score.PaymentReliability))
	table.Append("Execution Quality", fmt.Sprintf("%.1f/5", score.ExecutionQuality))
	table.Append("Communication", fmt.Sprintf("%.1f/5", score.Communication))
	table.Append("Resource Usage", fmt.Sprintf("%.1f/5", score.ResourceUsage))
	table.Render()

	fmt.Println()
	fmt.Printf("Transactions: %d total, %d completed, %d disputed\n",
		score.TotalTransactions, score.CompletedTransactions, score.DisputedTransactions)

	if score.LastAnchorBlock != nil && *score.LastAnchorBlock > 0 {
		fmt.Printf("Last Blockchain Anchor: Block %d\n", *score.LastAnchorBlock)
	}

	return nil
}

func runLBTASRateUser(cmd *cobra.Command, args []string) error {
	if rateTransactionID == "" {
		return fmt.Errorf("--transaction is required")
	}

	rating := lbtas.LBTASRating{
		Score:    rateScore,
		Category: rateCategory,
		Feedback: rateFeedback,
	}
	if err := lbtas.ValidateRating(rating); err != nil {
		return err
	}

	fmt.Printf("Rating submitted: %d/5 (%s)\n", rateScore, rateCategory)
	if rateFeedback != "" {
		fmt.Printf("Feedback: %s\n", rateFeedback)
	}
	fmt.Printf("Transaction: %s\n", rateTransactionID)

	return nil
}

func runLBTASRateProvider(cmd *cobra.Command, args []string) error {
	if rateTransactionID == "" {
		return fmt.Errorf("--transaction is required")
	}

	rating := lbtas.LBTASRating{
		Score:    rateScore,
		Category: rateCategory,
		Feedback: rateFeedback,
	}
	if err := lbtas.ValidateRating(rating); err != nil {
		return err
	}

	fmt.Printf("Rating submitted: %d/5 (%s)\n", rateScore, rateCategory)
	if rateFeedback != "" {
		fmt.Printf("Feedback: %s\n", rateFeedback)
	}
	fmt.Printf("Transaction: %s\n", rateTransactionID)

	return nil
}

// Ensure strconv is used (for future numeric parsing in CLI args)
var _ = strconv.Itoa
