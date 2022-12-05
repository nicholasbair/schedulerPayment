package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"
	"log"
	"os"
	"schedulerPayment/core"
)

func main() {
	errEnv := godotenv.Load()
	if errEnv != nil {
		log.Fatal("Error loading .env file")
	}

	stripe.Key = os.Getenv("STRIPE_CLIENT_SECRET")
	core.InitIMDB()
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{Views: engine})

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Scheduling Routes ///////////////////////////////////////////////////////////////////////////////////////////////
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	scheduling := app.Group("/scheduling")
	// Receive the redirect from the scheduler, store details from the pending meeting, kick off payment workflow with Stripe
	scheduling.Get("/thank-you", func(ctx *fiber.Ctx) error {
		eventId := ctx.Query("event_id")
		pageSlug := ctx.Query("page_slug")
		editHash := ctx.Query("edit_hash")

		core.SavePendingMeeting(eventId, pageSlug, editHash)

		// Passing eventId here so the payment redirect can be associated with the event
		return ctx.Render("checkout", fiber.Map{"eventId": eventId})
	})

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Payment Routes //////////////////////////////////////////////////////////////////////////////////////////////////
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	payments := app.Group("/payments")
	payments.Post("/create-checkout-session", func(ctx *fiber.Ctx) error {
		// Grab the eventId from the form submission so that it can be included on the success/cancel redirect
		eventId := ctx.FormValue("eventId")
		params := &stripe.CheckoutSessionParams{
			Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
						Currency: stripe.String("usd"),
						ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
							Name: stripe.String("Meeting"),
						},
						UnitAmount: stripe.Int64(30000),
					},
					Quantity: stripe.Int64(1),
				},
			},
			// Include the eventId on the redirect so the payment can be associated to the correct event
			SuccessURL: stripe.String("http://localhost:8000/payments/success?eventId=" + eventId),
			CancelURL:  stripe.String("http://localhost:8000/payments/cancel?eventId=" + eventId),
		}

		s, err := session.New(params)

		if err != nil {
			return ctx.SendStatus(fiber.StatusInternalServerError)
		}

		return ctx.Redirect(s.URL)
	})

	payments.Get("/success", func(ctx *fiber.Ctx) error {
		eventId := ctx.Query("eventId")
		log.Println("Payment success: found eventId", eventId)
		core.GetAndAcceptPendingMeeting(eventId)

		return ctx.Render("success", nil)
	})

	payments.Get("/cancel", func(ctx *fiber.Ctx) error {
		eventId := ctx.Query("eventId")
		log.Println("Payment failure: found eventId", eventId)
		core.GetAndDeletePendingMeeting(eventId)

		return ctx.Render("cancel", nil)
	})

	log.Fatal(app.Listen(":8000"))
}
