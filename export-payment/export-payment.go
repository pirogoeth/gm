package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"gopkg.in/urfave/cli.v1"
)

const (
	monthSelect = "01/2006"
)

var (
	stripeKey string
	monthStr string
	exportCsv bool
)

type DateRange struct {
	Begin	time.Time
	End		time.Time
}

func NewDateRange(begin string) (*DateRange, error) {
	d, err := time.Parse(monthSelect, begin)
	if err != nil {
		return nil, err
	}

	return &DateRange{
		Begin: d,
		End: d.AddDate(0, 1, 0),
	}, nil
}

func (d *DateRange) ToUnixTS() (int64, int64) {
	return d.Begin.Unix(), d.End.Unix()
}

func main() {
	app := cli.NewApp()
	app.Name = "export-payments"
	app.Usage = "Export and filter charge data from Stripe"
	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name: "api-key",
			Usage: "Stripe API key",
			Destination: &stripeKey,
			EnvVar: "STRIPE_API_KEY",
		},
		cli.StringFlag{
			Name: "select-month",
			Usage: "Month to select for data export (eg., `01/2017`)",
			Value: "01/2017",
			Destination: &monthStr,
		},
		cli.BoolFlag{
			Name: "export-csv",
			Usage: "Export data as CSV instead of spewed objects",
			Destination: &exportCsv,
		},
	}
	app.Action = func(c *cli.Context) error {
		dr, err := NewDateRange(monthStr)
		if err != nil {
			panic(err)
		}

		charges := fetchCharges(dr)
		err = exportCharges(charges)
		if err != nil {
			panic(err)
		}

		return nil
	}

	app.Run(os.Args)
}

func fetchCharges(dr *DateRange) []stripe.Charge {
	scli := &client.API{}
	scli.Init(stripeKey, nil)

	var charges []stripe.Charge
	beginTs, endTs := dr.ToUnixTS()

	params := &stripe.ChargeListParams{}
	params.Filters.AddFilter("created", "gt", strconv.FormatInt(beginTs, 10))
	params.Filters.AddFilter("created", "lt", strconv.FormatInt(endTs, 10))
	i := scli.Charges.List(params)
	for i.Next() {
		charges = append(charges, *i.Charge())
	}

	return charges
}

func exportCharges(charges []stripe.Charge) error {
	if !exportCsv {
		spew.Dump(charges)
		return nil
	}

	w := csv.NewWriter(os.Stdout)

	// header row
	w.Write([]string{
		"id", "status", "amount", "invoice_num",
		"currency", "created_at", "failure_message",
		"failure_type", "gateway", "payment_type",
		"cc_brand", "cc_last_four", "cc_expiry", "payment_info",
		"risk_level", "outcome_network_status",
	})

	for _, c := range charges {
		var ccExpiry time.Time
		var ccBrand stripe.CardBrand
		var ccLastFour string
		var err error

		card := c.Source.Card
		if card != nil {
			ccExpiry, err = time.Parse(monthSelect, fmt.Sprintf("%02d/%d", card.Month, card.Year))
			if err != nil {
				panic(err)
			}

			ccBrand = card.Brand
			ccLastFour = card.LastFour
		} else {
			ccExpiry = time.Now()
			ccBrand = ""
			ccLastFour = ""
		}

		invoice := c.Invoice
		var invId string
		if invoice == nil {
			invId = ""
		} else {
			invId = invoice.ID
		}

		err = w.Write([]string{
			fmt.Sprintf("%s", c.ID),
			fmt.Sprintf("%s", c.Status),
			fmt.Sprintf("%d", c.Amount),
			fmt.Sprintf("%s", invId),
			fmt.Sprintf("%s", c.Currency),
			fmt.Sprintf("%d", c.Created),
			fmt.Sprintf("%s", c.FailMsg),
			fmt.Sprintf("%s", c.FailCode),
			"stripe",
			fmt.Sprintf("%s", c.Source.Type),
			fmt.Sprintf("%s", ccBrand),
			fmt.Sprintf("%s", ccLastFour),
			fmt.Sprintf("%d", ccExpiry.Unix()),
			fmt.Sprintf("%s", c.Source.Display()),
			fmt.Sprintf("%s", c.Outcome.RiskLevel),
			fmt.Sprintf("%s", c.Outcome.NetworkStatus),
		})

		w.Flush()

		if err != nil {
			return err
		}
	}

	return nil
}
