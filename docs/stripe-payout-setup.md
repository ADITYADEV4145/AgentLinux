# Stripe account and bank payout setup

Official documentation checked 2026-11-05. This describes the owner's merchant account, not a marketplace paying third-party sellers; ordinary Stripe merchant payouts are the relevant path, not an assumed Stripe Connect integration.

1. Create/sign in to your account at [Stripe Dashboard](https://dashboard.stripe.com/). Select your actual supported business country and legal business type.
2. Follow the account setup checklist and provide business/product information and requested identity/tax verification directly to Stripe. Configure customer-facing business name, support details, website and statement descriptor. [Official account setup](https://docs.stripe.com/get-started/account/set-up).
3. Open **Settings → Payout settings** (the Dashboard may call the related section bank accounts/currencies). Add your payout bank account. Required fields depend on bank country; settlement currency must match the payout configuration. Complete any requested verification yourself. [Official payout setup](https://docs.stripe.com/payouts).
4. Select an available payout schedule. Timing and permitted schedules depend on country/account; bank transfers are not immediate just because a customer pays. Stripe states that initial live payouts typically take 7–14 days, with account-specific exceptions; rely on your Dashboard's expected date.
5. Keep Agent Linux sandbox-only while testing. Sandbox payments do not transfer real money to your bank. Bank linking alone does not make our checkout/subscription implementation production-ready.

Never send bank details, Stripe passwords, identity documents or secret keys through this chat or commit them. The app must never collect raw card details. Bank entry, business verification and any acceptance of Stripe agreements are actions for the account owner in the secure Dashboard.

Before live launch, implement and test subscription Checkout, signed renewal/cancellation/payment-failure webhooks, idempotent monthly hour grants (including annual customers), accurate entitlement debit across simultaneous sessions, refund/proration policy, and account-specific tax treatment. Existing code has sandbox customer/setup/portal/manual invoice foundations, not functioning recurring 30-hour subscriptions. Public real cloud, terms/permission, cost gates, security and explicit live activation authorization remain outstanding. No bank was connected and no real funds moved as part of this code change.
