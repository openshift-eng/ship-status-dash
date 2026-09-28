import { Link, Typography, styled } from '@mui/material'
import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'

const MarkdownContainer = styled('div')(({ theme }) => ({
  '& > *:first-of-type': {
    marginTop: 0,
  },
  '& > *:last-child': {
    marginBottom: 0,
  },
  '& h1, & h2, & h3, & h4, & h5, & h6': {
    marginTop: theme.spacing(2),
    marginBottom: theme.spacing(1),
  },
  '& p': {
    marginTop: theme.spacing(0.5),
    marginBottom: theme.spacing(0.5),
  },
  '& ul, & ol': {
    marginTop: theme.spacing(0.5),
    marginBottom: theme.spacing(0.5),
    paddingLeft: theme.spacing(3),
  },
  '& li': {
    marginBottom: theme.spacing(0.25),
  },
  '& pre': {
    backgroundColor:
      theme.palette.mode === 'dark' ? theme.palette.grey[900] : theme.palette.grey[100],
    borderRadius: theme.spacing(0.5),
    padding: theme.spacing(1.5),
    overflowX: 'auto',
    marginTop: theme.spacing(1),
    marginBottom: theme.spacing(1),
  },
  '& pre code': {
    backgroundColor: 'transparent',
    padding: 0,
    fontSize: '0.8125rem',
    fontFamily: 'monospace',
  },
  '& code': {
    backgroundColor:
      theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[200],
    borderRadius: theme.spacing(0.25),
    padding: '0.125em 0.375em',
    fontSize: '0.875em',
    fontFamily: 'monospace',
  },
}))

const sanitizeSchema = {
  ...defaultSchema,
  protocols: {
    ...defaultSchema.protocols,
    href: ['http', 'https', 'mailto'],
  },
}

const markdownComponents: Components = {
  h1: ({ children }) => (
    <Typography variant="h5" component="h1" gutterBottom>
      {children}
    </Typography>
  ),
  h2: ({ children }) => (
    <Typography variant="h6" component="h2" gutterBottom>
      {children}
    </Typography>
  ),
  h3: ({ children }) => (
    <Typography variant="subtitle1" component="h3" gutterBottom fontWeight={600}>
      {children}
    </Typography>
  ),
  h4: ({ children }) => (
    <Typography variant="subtitle2" component="h4" gutterBottom fontWeight={600}>
      {children}
    </Typography>
  ),
  h5: ({ children }) => (
    <Typography variant="body1" component="h5" gutterBottom fontWeight={600}>
      {children}
    </Typography>
  ),
  h6: ({ children }) => (
    <Typography variant="body2" component="h6" gutterBottom fontWeight={600}>
      {children}
    </Typography>
  ),
  p: ({ children }) => (
    <Typography variant="body2" component="p">
      {children}
    </Typography>
  ),
  a: ({ href, children }) => (
    <Link href={href} target="_blank" rel="noopener noreferrer" underline="hover">
      {children}
    </Link>
  ),
}

interface MarkdownRendererProps {
  content: string
}

const MarkdownRenderer = ({ content }: MarkdownRendererProps) => {
  return (
    <MarkdownContainer data-testid="markdown-content">
      <ReactMarkdown
        rehypePlugins={[[rehypeSanitize, sanitizeSchema]]}
        components={markdownComponents}
      >
        {content}
      </ReactMarkdown>
    </MarkdownContainer>
  )
}

export default MarkdownRenderer
