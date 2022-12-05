# Payment Workflow + Nylas Scheduler
This is a rough POC of incorporating a payment workflow into the Nylas scheduler.

## General Workflow
1. Scheduler page with confirmation method manual and a thank-you redirect
2. Upon redirect, parse and save the page slug, event ID and edit hash
3. Redirect the user to Stripe for payment
4. On payment success, accept the meeting via the scheduler link on the organizer's behalf
5. On payment failure, delete the event from the organizer's calendar

## Run the POC
1. Start the [acceptUtility](https://github.com/nickbair-nylas/acceptUtility)
2. Start ngrok (or your preferred reverse proxy) on port 8000
3. Build and run `schedulerPayment`
4. Create a scheduler page with confirmation method = manual and configure a thank-you redirect for the `thank-you` route
5. Book a meeting

## TODO
[] - Error handling
[] - Add TTL to pending meetings, purge meetings after TTL to handle case where user never completes payment workflow
[] - Validation/security for redirects
[] - Store and lookup connected account's access token instead of using `.env`