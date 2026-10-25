# Soundcheck

Never miss a concert again. Soundcheck tracks your favourite artists and
emails you when they announce a show near you, and again right before
tickets go on sale.

Work in progress — built during reading week while learning Go.

## How it works
1. Sign up with your email, pick your city and the artists you follow
2. A background worker checks Ticketmaster for new shows every few hours
3. You get an email when a show is announced, plus reminders 24 h and
   15 min before tickets go on sale
4. Manage your artists anytime through a personal link, no passwords

## Features
- Any city covered by Ticketmaster (US, Canada, UK, Australia and more)
- Email notifications, with Telegram as an optional extra channel
- On-sale and presale reminders
- Cancellation and reschedule alerts

## Stack
Go (net/http, html/template, goroutines) · PostgreSQL · Ticketmaster
Discovery API · Resend · Docker · Railway

## Architecture

One Go binary runs both the website and the background worker,
each in its own goroutine. Deployed on Railway with a Postgres database.

    User ──► web ──► store (Postgres) ◄── worker ──► ticketmaster
                                            │
                                            └──► notify ──► Email / Telegram

## Project structure

    cmd/soundcheck/        entry point: loads config, wires everything, starts web + worker
    internal/config/       environment variables
    internal/core/         shared types (Event, User) and interfaces (Source, Notifier)
    internal/ticketmaster/ Ticketmaster Discovery API client (implements Source)
    internal/store/        Postgres access
    internal/web/          HTTP handlers and HTML templates
    internal/worker/       periodic concert scan and on-sale reminders
    internal/notify/       email and Telegram notifiers
    migrations/            SQL schema