# Astro Landing Page Replider

```sh
npm create astro@latest -- --template minimal
```

> 🧑‍🚀 **Seasoned astronaut?** Delete this file. Have fun!

## 🚀 Project Structure

Inside of your Astro project, you'll see the following folders and files:

```text
/
├── public/
├── src/
│   └── pages/
│       └── index.astro
└── package.json
```

Astro looks for `.astro` or `.md` files in the `src/pages/` directory. Each page is exposed as a route based on its file name.

There's nothing special about `src/components/`, but that's where we like to put any Astro/React/Vue/Svelte/Preact components.

Any static assets, like images, can be placed in the `public/` directory.

## 🧞 Commands

All commands are run from the root of the project, from a terminal:

| Command                   | Action                                           |
| :------------------------ | :----------------------------------------------- |
| `npm install`             | Installs dependencies                            |
| `npm run dev`             | Starts local dev server at `localhost:4321`      |
| `npm run build`           | Build your production site to `./dist/`          |
| `npm run preview`         | Preview your build locally, before deploying     |
| `npm run astro ...`       | Run CLI commands like `astro add`, `astro check` |
| `npm run astro -- --help` | Get help using the Astro CLI                     |
| `npm test`                | Run Vitest unit tests                            |
| `npm run test:lighthouse` | Run Lighthouse CI perf/SEO budget audit          |

## 📊 Lighthouse CI (performance budget)

`npm run test:lighthouse` runs [Lighthouse CI](https://github.com/GoogleChrome/lighthouse-ci)
against the built `./dist/` (`/en/` and `/pt/` home pages) using the desktop preset.
Config lives in `lighthouserc.json`.

Budget assertions (the command **fails** if violated, which fails the `Deploy Web`
CI `test` job):

- **Performance** score must be `>= 0.80`
- **SEO** score must be `>= 0.95`

Run `npm run build` first locally (CI builds before auditing). Requires a Chrome
install; GitHub `ubuntu-latest` runners ship one.

## 👀 Want to learn more?

Feel free to check [our documentation](https://docs.astro.build) or jump into our [Discord server](https://astro.build/chat).
