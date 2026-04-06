# Stimulus + ESBuild + Turbo Setup

## How It Works

When you create a Rails app with `--javascript=esbuild`, Rails installs:

- `jsbundling-rails` gem — bridges Rails asset pipeline to esbuild
- `esbuild` npm package — the JavaScript bundler
- `@hotwired/turbo-rails` — Turbo (page navigation, frames, streams)
- `@hotwired/stimulus` — Stimulus (sprinkles of JS behavior on HTML)

ESBuild compiles `app/javascript/application.js` into `app/assets/builds/application.js`.

## File Structure

```
app/javascript/
  application.js              # Entry point — imports Turbo and Stimulus
  controllers/
    application.js            # Stimulus application instance
    index.js                  # Controller auto-loading
    hello_controller.js       # Example controller
```

## Entry Point: app/javascript/application.js

```javascript
import "@hotwired/turbo-rails"
import "./controllers"
```

## Stimulus Application: app/javascript/controllers/application.js

```javascript
import { Application } from "@hotwired/stimulus"

const application = Application.start()

// Configure Stimulus development experience
application.debug = false
window.Stimulus = application

export { application }
```

## Controller Index: app/javascript/controllers/index.js

```javascript
import { application } from "./application"

// Eager-load all controllers defined in this directory
import { eagerLoadControllersFrom } from "@hotwired/stimulus-loading"
eagerLoadControllersFrom("controllers", application)
```

If `@hotwired/stimulus-loading` is not available (some Rails versions), use
manual registration instead:

```javascript
import { application } from "./application"

import HelloController from "./hello_controller"
application.register("hello", HelloController)

// Add new controllers here as you create them
```

## Writing Stimulus Controllers

### Basic Controller

```javascript
// app/javascript/controllers/toggle_controller.js
import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["content"]

  toggle() {
    this.contentTarget.classList.toggle("hidden")
  }
}
```

Usage in Slim:

```slim
div data-controller="toggle"
  button data-action="click->toggle#toggle" Toggle
  div data-toggle-target="content"
    p This content toggles
```

### Controller with Values

```javascript
// app/javascript/controllers/countdown_controller.js
import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static values = { seconds: Number }

  connect() {
    this.start()
  }

  disconnect() {
    clearInterval(this.timer)
  }

  start() {
    this.timer = setInterval(() => {
      this.secondsValue--
      if (this.secondsValue <= 0) clearInterval(this.timer)
    }, 1000)
  }

  secondsValueChanged() {
    this.element.textContent = this.secondsValue
  }
}
```

## Turbo Frames

```slim
= turbo_frame_tag "user_profile" do
  / Content that can be lazily loaded or replaced
  p = @user.name
```

## Turbo Streams

```slim
/ app/views/comments/create.turbo_stream.slim
= turbo_stream.append "comments" do
  = render @comment
```

## ESBuild Configuration

The default `package.json` build script:

```json
{
  "scripts": {
    "build": "esbuild app/javascript/*.* --bundle --sourcemap --format=esm --outdir=app/assets/builds --public-path=/assets"
  }
}
```

For watch mode (in Procfile.dev):

```
js: yarn build --watch
```

## Adding New npm Packages

```bash
yarn add <package-name>
```

Then import in your JavaScript files. ESBuild handles the bundling.

## Troubleshooting

- **Controllers not connecting**: Check browser console for errors. Verify the
  controller filename matches the `data-controller` attribute (e.g.,
  `hello_controller.js` maps to `data-controller="hello"`).
- **Changes not reflecting**: Make sure `yarn build --watch` is running (via
  `bin/dev` or the Procfile.dev js process).
- **Module not found errors**: Run `yarn install` to ensure all npm dependencies
  are installed.
- **Turbo not working**: Check that `import "@hotwired/turbo-rails"` is in
  `application.js` and that the layout includes the JavaScript tag with
  `type: "module"`.
