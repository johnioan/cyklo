# Cyklo changelog

The in-app list (`app/web/changelog.js`, shown after an update and in Settings) is the Greek/English source; this file is a short English copy. Add an entry to both for every new version.

## 0.2.0
- New amount type "Installment plan": fixed monthly amount, number of installments (1-360) and first-installment date. The row shows "Installment 4 of 12"; the optional purchase price shows the extra cost of the installments (the difference, not an interest rate; "Interest-free" when zero). A down payment is not part of the plan. After the last installment the item becomes "Completed" and is left out of month totals, reminders, upcoming payments and the active count; its history stays in reports and the calendar.
- Chart "Expenses by month": click or tap a column to show the month and amount; a second click clears it.
- Calendar on narrow screens (phones): tapping a day with entries opens a window with all of its entries.
- "Subscriptions" renamed to "Recurring" (menu, titles, forms, messages, exports). Stored data and backups are unchanged and compatible.

## 0.1.8
- Repeated reminder when a variable-amount bill is missing: repeat interval (1, 2, 3, 5 or 7 days; default 3) and maximum number of reminders (1, 2, 3 or 5; default 3) in Settings → Notifications.
- Fix: a reminder that no channel could deliver is no longer marked as sent.
- Home: upcoming payments open the subscription, the "Next payment" tile shows it in a window and the "Active subscriptions" tile opens the active list; saving or cancelling returns to the screen the form was opened from (also from the Calendar).
- Info (i) buttons in the reminder settings; warning in Backups that the file contains the Pushover keys unencrypted.
- Version indicator at the bottom of Home that opens the release notes, and a small surprise next to it.

## 0.1.7
- After every update a "What is new" window lists the changes of the version; the full list is in Settings.

## 0.1.6
- Simplified the "Backups" section in Settings; removed the "DSM login" option.

## 0.1.5
- Company initial in the coloured square of each subscription.
- Custom categories and payment methods, also from the subscription form.
- Email notifications hidden for now; Pushover unchanged.

## 0.1.4
- Updated package developer and description.

## 0.1.2
- Data protection during package upgrades.

## 0.1.1
- Dates chosen as day, month and year.

## 0.1.0
- First release.
