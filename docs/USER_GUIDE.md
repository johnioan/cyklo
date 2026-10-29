# Cyklo – User Guide

Cyklo tracks recurring payments: fixed amounts (e.g. streaming), variable-amount bills and installment plans, shows what is due next, and summarises spending.

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

The screenshots use sample data with fictional companies.

The navigation contains: **Home**, **Recurring**, **Reports**, **Calendar**, **Settings**, and a button for a **New recurring payment**.

### Home

![Home](screenshots/home.png)
- **This month**: payments in the current month.
- **Next 12 months**: known amounts for the next 12 months.
- **Active**: paused ones are excluded.
- **Next payment**: the closest upcoming charge.
- Clicking an upcoming payment opens the item; saving or cancelling returns to the screen it was opened from. The **Next payment** tile shows that item in a window, and the **Active** tile opens the list of active ones.
- **Upcoming payments**, **Spending by category** and **Spending by month** (last 12 months); click or tap a column to show its month and amount, click again to clear.
- The version number at the bottom opens the release notes of all versions.
- A notice appears when variable-amount items are waiting for a bill to be entered.

### Recurring

![Recurring](screenshots/subscriptions.png)
A list of all entries with tabs **All / Active / Paused / Cancelled / Completed**, a search box (company or service) and a category filter. Each row shows a coloured tile with the first letter of the company, the next payment date and the amount. Select a row to edit it.

### Reports

![Reports](screenshots/reports.png)
- **Period**: by month or by year.
- **Items** included: active only, active and paused, or all.
- **View** grouped by payment method, company or category. Select a row to see its details or jump to the matching items.
- **Export** the current report to Excel or CSV.

### Calendar

![Calendar](screenshots/calendar.png)
A month view of payments. Days with a known amount and days awaiting a bill are marked differently. Select a day to list its payments; the month total is shown below the calendar. On narrow screens (phones) selecting a day with entries opens a window with all of them.

## 4. Adding a recurring payment

![New recurring payment](screenshots/new-subscription.png)

Press **New recurring payment** and fill in:

| Field | Meaning |
|---|---|
| Company, Service | e.g. "Netflix", "Premium" |
| Category | Each item belongs to one category. Choose **New category…** to create one on the spot. |
| Amount type | **Fixed amount**: always the same (streaming, software). **Variable amount**: changes with each bill (electricity, water, phone). **Installment plan**: a fixed amount for a set number of installments (section 5). |
| Cost | Only for fixed amounts. |
| Billing frequency | Weekly, monthly, quarterly, semi-annual or annual. |
| Start date | The date of one payment (the first or the latest). Following payments are calculated from it. If the day is 29, 30 or 31, shorter months use their last day. |
| Next payment | Calculated automatically from the start date and frequency. It can be set manually with **Change**; **Calculate automatically** restores the calculation. |
| Payment method | Credit card, PayPal, Apple Pay or a custom method. For a credit card, a friendly card name can be selected. |
| Reminder | How many days before the payment a notification is sent, or none. |
| Notes, Status | Status is **Active**, **Paused** or **Cancelled**. |

Cancelling is preferable to deleting: a cancelled item keeps its history, while **Delete** removes the item and its history permanently.

## 5. Variable-amount bills and installment plans

A variable-amount item has no fixed cost. When a bill arrives:

1. Open the item and press **Enter amount**.
2. Type the **bill amount** and save. Alternatively choose **No charge this period** to skip that period.

Until an amount is entered the entry is shown as **Awaiting bill**, and as **Bill missing** when the expected date has passed. Totals and reports include only the amounts that have been entered.

**Installment plan.** Choose the amount type **Installment plan** and enter the installment amount, the number of installments (1 to 360) and the date of the first installment; installments are monthly. The optional **Purchase price** shows the **Extra cost of installments**: the total of the installments minus the purchase price (a difference in money, not an interest rate; **Interest-free** when it is zero). A down payment is not part of the plan. The number of paid installments follows from the date of the first one, and the row shows e.g. "Installment 4 of 12". After the last installment the item becomes **Completed** (tab **Completed**): it is left out of month totals, reminders, upcoming payments and the active count, while its history remains in reports and the calendar.

## 6. Categories, payment methods and cards (Settings)

- **Categories**: add, rename or delete. A category that has items can only be deleted after choosing another category to move them to. The last remaining category cannot be deleted.
- **Payment methods**: the three default methods cannot be changed. Custom methods (for example "Bank transfer" or "Google Pay") can be added, renamed and deleted; if a method is in use, another one must be chosen for the affected items.
- **Cards**: friendly names only (for example "Personal card"). **Card numbers and other banking details are never stored.** Names with 8 or more digits are rejected.

## 7. Notifications (Pushover)

Reminders are sent through [Pushover](https://pushover.net), a third-party service for phone notifications. Pushover is not part of Cyklo, is not free of conditions (its own pricing and trial terms apply) and the project is not affiliated with it.

1. Create a Pushover account and install its app on the phone.
2. Copy the **User Key** from the Pushover dashboard.
3. In the Pushover website create an **Application / API Token** (any name, for example "Cyklo").
4. In Cyklo open **Settings → Notifications → Pushover** and enter the User Key and the API Token.
5. Press **Send test notification** to confirm that it reaches the phone.
6. Set the **Default reminder** (days before payment), the **Send time**, and optionally **Remind when a variable-amount bill is missing**. When that option is on, **Repeat reminder every** (1, 2, 3, 5 or 7 days) and **Maximum number of reminders** (1, 2, 3 or 5) control how often the missing-bill reminder is repeated; the first one is sent the day after the bill date, and repetition stops once the amount is entered or the bill is skipped.

The token is stored in the app's data file on the NAS and is not displayed again after saving. The data file is not encrypted. Each item can override the reminder setting in its own form.

Email reminders are not available in this version.

## 8. Backup and restore

**Settings → Backups**
- **Download backup** saves all data to a JSON file. Keep copies somewhere other than the NAS volume.
- **Restore from backup** replaces the current data with the contents of a backup file.

**A backup file contains all data, including the Pushover User Key and API Token, without encryption.** Keep it private and do not share it or post it online.

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
