# Slim Templating Setup

## Generator Configuration

Add to `config/application.rb` inside the `Application` class:

```ruby
config.generators do |g|
  g.template_engine :slim
end
```

This ensures all future generators (controllers, scaffolds, Devise views) produce
`.html.slim` files instead of `.html.erb`.

## Layout Conversion

Convert `app/views/layouts/application.html.erb` to `application.html.slim`:

```slim
doctype html
html
  head
    title= content_for(:title) || "AppName"
    meta name="viewport" content="width=device-width,initial-scale=1"
    meta name="apple-mobile-web-app-capable" content="yes"
    = csrf_meta_tags
    = csp_meta_tag
    = stylesheet_link_tag "tailwind", "inter-font", "data-turbo-track": "reload"
    = stylesheet_link_tag "application", "data-turbo-track": "reload"
    = javascript_include_tag "application", "data-turbo-track": "reload", type: "module"
  body
    - if notice.present?
      p.py-2.px-3.bg-green-50.text-green-500.font-medium.rounded-lg.mb-4 = notice
    - if alert.present?
      p.py-2.px-3.bg-red-50.text-red-500.font-medium.rounded-lg.mb-4 = alert
    = yield
```

**Delete** the old `application.html.erb` after creating the Slim version.

## Mailer Layout

If `app/views/layouts/mailer.html.erb` exists, convert to `mailer.html.slim`:

```slim
html
  body
    = yield
```

Also convert `mailer.text.erb` to `mailer.text.slim`:

```slim
= yield
```

## Slim Syntax Quick Reference

For anyone unfamiliar with Slim:

```slim
/ This is a comment
h1 Hello World
p.text-gray-600 A paragraph with a Tailwind class
div#main.container
  = some_helper_method
  - if condition
    p Conditional content
  = link_to "Click", some_path, class: "btn"
  = render "shared/partial"
```

## Troubleshooting

- **Devise views generated as .erb**: Make sure `slim-rails` is in the Gemfile and
  `bundle install` has been run BEFORE running `rails generate devise:views`.
  If already generated as .erb, either delete and regenerate, or manually convert.
- **Indentation errors**: Slim is indentation-sensitive (like Python). Use 2 spaces
  consistently. Never mix tabs and spaces.
