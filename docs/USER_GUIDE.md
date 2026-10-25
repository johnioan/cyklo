# Cyklo – User Guide

Cyklo tracks subscriptions (fixed amount) and recurring bills (variable amount), shows what is due next, and summarises spending.

> **Provided "as is", without warranty of any kind** (MIT License). Tested only by its author on one Synology DS-918+ with DSM 7.4. Make a backup before every update.

## 1. Installation

1. Download `cyklosub-<version>.spk` from the repository's **Releases** page.
2. In DSM open **Package Center → Manual Install**, select the file and follow the wizard.
3. Open **Cyklo** from the DSM desktop, or go to `http://<NAS address>:8765/` in a browser.

**Security note.** The app has no login of its own. Anyone who can reach port 8765 can read and change the data. Use it only from a trusted network (home LAN or a VPN).

**Updating.** Make a backup first (section 8), then install the new SPK over the old one with Manual Install. After the update the app shows a "What is new" window listing the changes. The full list is available at any time in **Settings → What is new (all versions)**.

## 2. First start

On the first launch a short setup wizard asks for:

1. **Language** (Greek or English)
2. **Currency** (Euro, US dollar, pound sterling or Swiss franc)
3. **Notifications** (optional): how many days before each payment a reminder is sent

All three can be changed later in **Settings**. The currency applies to the whole application; amounts are not converted.

## 3. Main screens

The navigation contains: **Home**, **Subscriptions**, **Reports**, **Calendar**, **Settings**, and a button for a **New subscription**.

### Home
- **This month**: payments in the current month.
- **Next 12 months**: known amounts for the next 12 months.
- **Active subscriptions**: paused ones are excluded.
- **Next payment**: the closest upcoming charge.
- **Upcoming payments**, **Spending by category** and **Spending by month** (last 12 months).
- A notice appears when variable-amount subscriptions are waiting for a bill to be entered.

### Subscriptions
A list of all entries with tabs **All / Active / Paused / Cancelled**, a search box (company or service) and a category filter. Each row shows a coloured tile with the first letter of the company, the next payment date and the amount. Select a row to edit it.

### Reports
- **Period**: by month or by year.
- **Subscriptions** included: active only, active and paused, or all.
- **View** grouped by payment method, company or category. Select a row to see its details or jump to the matching subscriptions.
- **Export** the current report to Excel or CSV.

### Calendar
A month view of payments. Days with a known amount and days awaiting a bill are marked differently. Select a day to list its payments; the month total is shown below the calendar.

## 4. Adding a subscription

Press **New subscription** and fill in:

| Field | Meaning |
|---|---|
| Company, Service | e.g. "Netflix", "Premium" |
| Category | Each subscription belongs to one category. Choose **New category…** to create one on the spot. |
| Amount type | **Fixed amount**: always the same (streaming, software). **Variable amount**: changes with each bill (electricity, water, phone). |
| Cost | Only for fixed amounts. |
| Billing frequency | Weekly, monthly, quarterly, semi-annual or annual. |
| Start date | The date of one payment (the first or the latest). Following payments are calculated from it. If the day is 29, 30 or 31, shorter months use their last day. |
| Next payment | Calculated automatically from the start date and frequency. It can be set manually with **Change**; **Calculate automatically** restores the calculation. |
| Payment method | Credit card, PayPal, Apple Pay or a custom method. For a credit card, a friendly card name can be selected. |
| Reminder | How many days before the payment a notification is sent, or none. |
| Notes, Status | Status is **Active**, **Paused** or **Cancelled**. |

Cancelling is preferable to deleting: a cancelled subscription keeps its history, while **Delete** removes the subscription and its history permanently.

## 5. Variable-amount bills

A variable-amount subscription has no fixed cost. When a bill arrives:

1. Open the subscription and press **Enter amount**.
2. Type the **bill amount** and save. Alternatively choose **No charge this period** to skip that period.

Until an amount is entered the entry is shown as **Awaiting bill**, and as **Bill missing** when the expected date has passed. Totals and reports include only the amounts that have been entered.

## 6. Categories, payment methods and cards (Settings)

- **Categories**: add, rename or delete. A category that has subscriptions can only be deleted after choosing another category to move them to. The last remaining category cannot be deleted.
- **Payment methods**: the three default methods cannot be changed. Custom methods (for example "Bank transfer" or "Google Pay") can be added, renamed and deleted; if a method is in use, another one must be chosen for the affected subscriptions.
- **Cards**: friendly names only (for example "Personal card"). **Card numbers and other banking details are never stored.** Names with 8 or more digits are rejected.

## 7. Notifications (Pushover)

Reminders are sent through [Pushover](https://pushover.net), a third-party service for phone notifications. Pushover is not part of Cyklo, is not free of conditions (its own pricing and trial terms apply) and the project is not affiliated with it.

1. Create a Pushover account and install its app on the phone.
2. Copy the **User Key** from the Pushover dashboard.
3. In the Pushover website create an **Application / API Token** (any name, for example "Cyklo").
4. In Cyklo open **Settings → Notifications → Pushover** and enter the User Key and the API Token.
5. Press **Send test notification** to confirm that it reaches the phone.
6. Set the **Default reminder** (days before payment), the **Send time**, and optionally **Remind when a variable-amount bill is missing**.

The token is stored on the NAS and is not displayed again after saving. Each subscription can override the reminder setting in its own form.

Email reminders are not available in this version.

## 8. Backup and restore

**Settings → Backups**
- **Download backup** saves all data to a JSON file. Keep copies somewhere other than the NAS volume.
- **Restore from backup** replaces the current data with the contents of a backup file.

Make a backup before every update and before uninstalling.

## 9. Export

Reports can be exported to **Excel** or **CSV** from the Reports screen.

## 10. Troubleshooting

| Problem | What to check |
|---|---|
| The page does not open | Confirm the package is running in DSM Package Center, and that the address is `http://<NAS address>:8765/`. |
| "The server cannot be reached" | The package was stopped. Start it again in Package Center. |
| Test notification fails | Re-check the User Key and API Token, and that the NAS has internet access. |
| A date looks wrong | If the start day is 29, 30 or 31, months with fewer days use their last day. Set the next payment manually if needed. |
| Data seems missing after an update | Use **Restore from backup** with the file saved before the update. |

## 11. Limitations

- One user per installation, no login. Reach it only from a trusted network.
- No currency conversion; one currency for the whole application.
- Email reminders and DSM sign-in are not available.
- Packaged for Synology (`x86_64`, `apollolake`). Other platforms are untested.
