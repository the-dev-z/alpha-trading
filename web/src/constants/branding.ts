// Alpha Trading Branding Constants

// Official links
export const OFFICIAL_LINKS = {
  twitter: '',
  telegram: '',
  github: 'https://github.com/the-dev-z/nofx',
} as const

// Brand info
export const BRAND_INFO = {
  name: 'Alpha Trading',
  tagline: 'Quantitative Trading Platform',
  version: '1.0.0',
  social: {
    x: () => OFFICIAL_LINKS.twitter,
    tg: () => OFFICIAL_LINKS.telegram,
    gh: () => OFFICIAL_LINKS.github,
  }
} as const
