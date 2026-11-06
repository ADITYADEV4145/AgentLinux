# Proposed launch pricing — not an active offer

Proposal dated 2026-11-05. Local software usage stays free. Do not enable live billing, publish these as finalized prices, or promise profit based on this document. The editable website catalog is `internal/console/assets/pricing.json`; it does not modify the sandbox billing ledger or Stripe products.

| Proposed size | Resource target | Candidate USD/hour | Ten hours |
| --- | --- | --- | --- |
| Desktop | 2 vCPU, 4 GiB RAM, 24 GiB ephemeral disk | $0.59 | $5.90 |
| Builder (planned) | 4 vCPU, 8 GiB RAM, 48 GiB ephemeral disk | $1.19 | $11.90 |

These prices sell an agent/human desktop workflow, not raw CPU alone. No subscription or fictitious team tier. No active public free trial. Consider a one-time 15-minute Desktop trial only after abuse controls and a funded trial pool exist; never promise unlimited free compute. Local hardware, electricity and third-party agent/model costs remain the user's responsibility.

Proposed runtime semantics: per-second from verified ready until confirmed termination; idle time counts, failures before ready do not count as customer runtime. Provider provisioning/failure charges are our exposure. No persistent disk plan yet; stopped sessions discard guest data. Payment processing included in target; taxes additional where applicable. A measured streaming/egress allowance, visible pre-cap warnings and an enforceable stop policy must be chosen before sales. Until then no claim of unlimited bandwidth, no surprise network surcharge and no purchasable cloud CTA. Aggregate invoices/credits rather than separate card charges per short session; final funding/refund rules still need review.

## What would make this profitable?

Use the existing editable scenarios, now with the exact website catalog:

```sh
python3 scripts/cost_model.py --catalog internal/console/assets/pricing.json
```

The model includes CPU, RAM, disk lifetime, streaming/other egress, provider overhead, trial/orphan waste, database/control plane, support, and payment fees. CPU/RAM/disk scale linearly for Builder; support/control assumptions remain fixed. All are assumptions, not measured bills or a contractual provider quote. Payment assumptions are 2.9% + $0.30 per aggregate invoice across ten machine-hours, not a verified Stripe quote.

| Scenario | Desktop estimated cost/h including payments | Desktop estimated margin | Builder estimated cost/h | Builder estimated margin |
| --- | ---: | ---: | ---: | ---: |
| Low overhead / 2,000 billed hours monthly | $0.2745 | 53.5% | $0.4278 | 64.1% |
| Base / 500 billed hours monthly | $0.7643 | **−29.5%** | $0.9319 | 21.7% |
| High egress / 100 billed hours monthly | $5.0224 | **loss** | $5.2292 | **loss** |

Therefore $0.59 is an aggressive target, not a guaranteed profitable launch price. The current 60% model target is not met by Desktop even in the low-overhead case. A proposed **minimum 40% contribution margin gate** permits Desktop non-payment cost at most $0.30689/h with these fee assumptions; the base case fails. Company net profit additionally requires covering acquisition, engineering, legal, taxes and fixed expenses not comprehensively modeled here. Do not hide early low-utilization losses behind a raw-compute margin.

Before activating: run real provider measurements at representative utilization and stream activity; resolve commercial resale rights; include failed/orphaned compute; validate merchant-country fees/tax treatment; choose a bounded bandwidth policy; reject or raise prices if margin fails. Builder also needs an actual selectable provider profile and billing size validation before becoming available.

## Official cost context and uncertainty

[Daytona's pricing page](https://www.daytona.io/pricing) lists $0.0504/vCPU-hour and $0.0162/GiB-hour, with disk $0.000108/GiB-hour after its stated free allocation. A full 2/4 reservation is therefore about $0.1656/h **before disk, traffic and our product expenses**. It is a cost reference, not an implemented adapter or a comparable full desktop retail offer.

The current [Daytona v2 billing documentation](https://www.daytona.io/docs/en/v2/billing/) adds resource guarantees/measurement and placement multipliers; it describes per-minute metering while the pricing page says per-second. A pinned region can multiply base resource rates. Resolve the exact API/version/region contract before assuming these headline rates. No paid Daytona resources were provisioned. Existing E2B adapter commercial permission and hardware acceptance remain gates documented in the cloud report; these targets are not validated against an enabled E2B account.

We cannot honestly call this the cheapest product in the market: raw sandboxes, managed graphical desktops, persistence, quotas and support are different offerings. Sustainable value comes from the integrated desktop, MCP, human handoff and transparent limits.
