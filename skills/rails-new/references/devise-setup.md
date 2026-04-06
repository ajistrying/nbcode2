# Devise Authentication Setup

## Installation Steps

1. Run `rails generate devise:install`
2. Follow the on-screen instructions from the installer

## Required Configuration

### Mailer URL (config/environments/development.rb)

```ruby
config.action_mailer.default_url_options = { host: "localhost", port: 3000 }
```

### Letter Opener for Dev Emails (config/environments/development.rb)

```ruby
config.action_mailer.delivery_method = :letter_opener
config.action_mailer.perform_deliveries = true
```

### Flash Messages in Layout

The Slim layout should include notice and alert flash messages. These are already
included in the main SKILL.md layout template.

## Generate User Model

```bash
rails generate devise User
rails db:migrate
```

The default Devise modules (database_authenticatable, registerable, recoverable,
rememberable, validatable) are sufficient for initial setup.

## Generate Views

```bash
rails generate devise:views
```

If the generator produces `.erb` files instead of `.slim`, convert the key views:

- `devise/sessions/new` (sign in)
- `devise/registrations/new` (sign up)
- `devise/registrations/edit` (edit profile)
- `devise/passwords/new` (forgot password)
- `devise/passwords/edit` (reset password)
- `devise/shared/_links` (shared navigation links)
- `devise/shared/_error_messages` (form errors partial)

### Sample Slim Conversion: Sign In (devise/sessions/new.html.slim)

```slim
h2 Sign In

= form_for(resource, as: resource_name, url: session_path(resource_name)) do |f|
  .field.mb-4
    = f.label :email, class: "block text-sm font-medium text-gray-700"
    = f.email_field :email, autofocus: true, autocomplete: "email", class: "mt-1 block w-full rounded-md border-gray-300 shadow-sm"

  .field.mb-4
    = f.label :password, class: "block text-sm font-medium text-gray-700"
    = f.password_field :password, autocomplete: "current-password", class: "mt-1 block w-full rounded-md border-gray-300 shadow-sm"

  - if devise_mapping.rememberable?
    .field.mb-4
      = f.label :remember_me, class: "inline-flex items-center"
        = f.check_box :remember_me, class: "mr-2"

  .actions
    = f.submit "Sign In", class: "bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"

= render "devise/shared/links"
```

## Troubleshooting

- If `rails db:create` fails: PostgreSQL may not be running, or the role may not exist.
  Do NOT attempt to start services. Inform the user.
- If Devise views generate as `.erb`: The `slim-rails` gem may need to be installed
  before running the Devise view generator. Run `bundle install` first.
