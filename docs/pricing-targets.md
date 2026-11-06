# Owner-selected pricing — cloud sales not active

Selected by the owner on 2026-11-05. Local software usage stays free. Displaying these prices does not activate billing, create Stripe products or sell a functioning hosted subscription. The canonical editable catalog is `internal/console/assets/pricing.json`; it is separate from sandbox billing ledger rates.

| Size | Resources | Pay as you go | Monthly | Annual |
| --- | --- | --- | --- | --- |
| Desktop | 2 vCPU, 4 GiB RAM, 24 GiB ephemeral disk target | $1/hour | $20 for 30 hours/month | $200 upfront for 30 hours/month |
| Builder (planned) | 4 vCPU, 8 GiB RAM, 48 GiB ephemeral disk target | $2/hour | $40 for 30 hours/month | $450 upfront for 30 hours/month |

Annual is interpreted as the same monthly allowance for twelve months: 360 hours total, released 30 each month, not all available upfront. Desktop saves $40/year versus twelve monthly payments; Builder saves $30/year. Unused hours do not roll over in the displayed policy. Allowances count aggregate machine-hours: two machines running one hour consume two hours. Larger-resource profiles and recurring subscription entitlements are still planned, not implemented checkout products.

Selected annual effective prices at full allowance consumption are about $0.5556/hour Desktop and $1.25/hour Builder. Monthly equivalents are about $0.6667/hour and $1.3333/hour. Annual cash received upfront is not all earned profit: compute must still be provided throughout the term. Subscription proration, cancellation, refunds, renewal anchors, payment-failure handling and monthly grant accounting need implementation and tests before sale.

Proposed safety policy: stop at exhausted included hours unless the customer explicitly enables additional usage at the selected hourly rate. There is no automatic overage billing in this implementation. Runtime semantics remain per-second from verified ready until confirmed termination; idle counts, failure before ready does not. Ephemeral disk is included while running; stopped cloud sessions discard files. No persistent disk plan or active public trial. Payment processing is intended to be included in price; taxes additional where applicable. Bandwidth/streaming allowance and provider commercial rights must be resolved before sale; no unlimited-bandwidth promise.

## Profit check before activation

```sh
python3 scripts/cost_model.py --catalog internal/console/assets/pricing.json
```

The model now evaluates hourly, monthly and annual options at full allowance consumption using the exact website catalog. It includes compute, RAM, disk, egress, overhead, trials/orphans, support, database/control plane and assumed processing fees. Subscription options retain the conservative existing invoice-fee allocation rather than pretending real merchant fees were validated.

Under the low-overhead assumptions, Desktop annual revenue of about $0.5556/hour is above estimated costs. Under the base low-utilization scenario, that same annual price loses money. The higher $1 hourly rate does not remove annual-plan exposure. No price guarantees profit across utilization, support and streaming scenarios. Company expenses and taxes also need budgeting.

Before selling, require measured costs below the revenue for every option at full usage, with a proposed minimum 40% contribution margin. Validate country-specific processing fees, average utilization, failed/orphaned provider resources, support and bounded egress. The tool always reports production activation as false. No catalog update changes existing sandbox reservations or invoice rate references.

## Receiving money

See [Stripe bank/payout setup](stripe-payout-setup.md). Bank details belong in Stripe's secure Dashboard, not source code or this chat. Production activation and subscription implementation remain separate release gates.

## Official cost context and uncertainty

[Daytona's pricing page](https://www.daytona.io/pricing) lists $0.0504/vCPU-hour and $0.0162/GiB-hour, with disk $0.000108/GiB-hour after its stated free allocation. A full 2/4 reservation is therefore about $0.1656/h **before disk, traffic and our product expenses**. It is a cost reference, not an implemented adapter or a comparable full desktop retail offer.

The current [Daytona v2 billing documentation](https://www.daytona.io/docs/en/v2/billing/) adds resource guarantees/measurement and placement multipliers; it describes per-minute metering while the pricing page says per-second. A pinned region can multiply base resource rates. Resolve the exact API/version/region contract before assuming these headline rates. No paid Daytona resources were provisioned. Existing E2B adapter commercial permission and hardware acceptance remain gates documented in the cloud report; these targets are not validated against an enabled E2B account.

We cannot honestly call this the cheapest product in the market: raw sandboxes, managed graphical desktops, persistence, quotas and support are different offerings. Sustainable value comes from the integrated desktop, MCP, human handoff and transparent limits.
