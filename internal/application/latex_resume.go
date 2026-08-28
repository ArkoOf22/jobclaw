package application

import (
	"fmt"
	"strings"
)

// ResumeContact is the header identity of a resume. It comes from config, never
// from the model, so it is passed in rather than generated.
type ResumeContact struct {
	Name     string
	Phone    string
	Email    string
	LinkedIn string
	GitHub   string
	LeetCode string
}

// ResumeEducation is one education entry. Stable candidate fact, sourced from
// config rather than tailored per job.
type ResumeEducation struct {
	Institution  string
	Dates        string
	DegreeAndGPA string
}

// ResumeSkillLine is one labelled row of the Technical Skills section, e.g.
// Label "Languages", Value "Go, Java, SQL".
type ResumeSkillLine struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ResumeExperience is one work-history entry.
type ResumeExperience struct {
	Company string   `json:"company"`
	Role    string   `json:"role"`
	Tech    string   `json:"tech"`
	Dates   string   `json:"dates"`
	Bullets []string `json:"bullets"`
}

// ResumeProject is one project entry.
type ResumeProject struct {
	Name    string   `json:"name"`
	Tech    string   `json:"tech"`
	GitHub  string   `json:"github"`
	Bullets []string `json:"bullets"`
}

// ResumeContent is the tailored body the model produces from the master resume.
// It deliberately excludes contact and education, which are fixed facts supplied
// from config.
type ResumeContent struct {
	Skills      []ResumeSkillLine  `json:"skills"`
	Achievement string             `json:"achievement_line"`
	Experience  []ResumeExperience `json:"experience"`
	Projects    []ResumeProject    `json:"projects"`
}

// textFields returns every free-text value in the tailored content, so a caller
// can run the fact guard over the whole thing before it is rendered.
func (c ResumeContent) textFields() string {
	var b strings.Builder

	for _, s := range c.Skills {
		b.WriteString(s.Label)
		b.WriteString(" ")
		b.WriteString(s.Value)
		b.WriteString("\n")
	}

	b.WriteString(c.Achievement)
	b.WriteString("\n")

	for _, e := range c.Experience {
		fmt.Fprintf(&b, "%s %s %s %s\n", e.Company, e.Role, e.Tech, e.Dates)

		for _, bullet := range e.Bullets {
			b.WriteString(bullet)
			b.WriteString("\n")
		}
	}

	for _, p := range c.Projects {
		fmt.Fprintf(&b, "%s %s\n", p.Name, p.Tech)

		for _, bullet := range p.Bullets {
			b.WriteString(bullet)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// latexPreamble is the fixed document preamble and macro set, taken verbatim from
// the candidate's chosen template. It compiles with pdflatex, which is why the
// pdfTeX-specific glyphtounicode lines are kept: they give the resulting PDF a
// proper ToUnicode map so an ATS can extract the text cleanly.
const latexPreamble = `\documentclass[letterpaper,11pt]{article}

\usepackage{latexsym}
\usepackage[empty]{fullpage}
\usepackage{titlesec}
\usepackage{marvosym}
\usepackage[usenames,dvipsnames]{color}
\usepackage{verbatim}
\usepackage{enumitem}
\usepackage[hidelinks]{hyperref}
\usepackage{fancyhdr}
\usepackage[english]{babel}
\usepackage{tabularx}
\usepackage{fontawesome5}
\usepackage{multicol}
\setlength{\multicolsep}{-3.0pt}
\setlength{\columnsep}{-1pt}
\input{glyphtounicode}

\pagestyle{fancy}
\fancyhf{}
\fancyfoot{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\addtolength{\oddsidemargin}{-0.6in}
\addtolength{\evensidemargin}{-0.5in}
\addtolength{\textwidth}{1.19in}
\addtolength{\topmargin}{-.7in}
\addtolength{\textheight}{1.4in}

\urlstyle{same}

\raggedbottom
\raggedright
\setlength{\tabcolsep}{0in}

\titleformat{\section}{
  \vspace{-4pt}\scshape\raggedright\large\bfseries
}{}{0em}{}[\color{black}\titlerule \vspace{-5pt}]

\pdfgentounicode=1

\newcommand{\resumeItem}[1]{
  \item\small{#1 \vspace{-2pt}}
}

\newcommand{\resumeSubheading}[4]{
  \vspace{-2pt}\item
    \begin{tabular*}{1.0\textwidth}[t]{l@{\extracolsep{\fill}}r}
      \textbf{#1} & \textbf{\small #2} \\
      \textit{\small#3} & \textit{\small #4} \\
    \end{tabular*}\vspace{-7pt}
}

\newcommand{\resumeProjectHeading}[2]{
    \item
    \begin{tabular*}{1.001\textwidth}{l@{\extracolsep{\fill}}r}
      \small#1 & \textbf{\small #2}\\
    \end{tabular*}\vspace{-7pt}
}

\renewcommand\labelitemi{$\vcenter{\hbox{\tiny$\bullet$}}$}

\newcommand{\resumeSubHeadingListStart}{\begin{itemize}[leftmargin=0.0in, label={}]}
\newcommand{\resumeSubHeadingListEnd}{\end{itemize}}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}\vspace{-5pt}}

\newcommand{\resumeEducationEntry}[4]{
  \item
  \begin{tabular*}{1.0\textwidth}[t]{l@{\extracolsep{\fill}}r}
    \textbf{#1} & \textbf{\small #2} \\
    \textit{\small #3} & \\
  \end{tabular*}
  \vspace{-2pt}
  {\small #4}
  \vspace{-6pt}
}
`

// latexReplacer escapes the characters that are special in LaTeX so that text
// drawn from a resume renders literally instead of being interpreted as markup.
// A stray percent sign in "92% improvement" would otherwise comment out the rest
// of the line.
var latexReplacer = strings.NewReplacer(
	`\`, `\textbackslash{}`,
	`&`, `\&`,
	`%`, `\%`,
	`$`, `\$`,
	`#`, `\#`,
	`_`, `\_`,
	`{`, `\{`,
	`}`, `\}`,
	`~`, `\textasciitilde{}`,
	`^`, `\textasciicircum{}`,
)

// escapeLaTeX makes a plain-text string safe to drop into a LaTeX document.
func escapeLaTeX(s string) string {
	return latexReplacer.Replace(strings.TrimSpace(s))
}

// RenderResumeLaTeX assembles a complete LaTeX document from the fixed contact
// and education facts and the tailored body content.
//
// Every dynamic value is escaped, except URLs, which are passed to \href raw
// (hyperref handles them) while their visible label is escaped.
func RenderResumeLaTeX(
	contact ResumeContact,
	education []ResumeEducation,
	content ResumeContent,
) string {
	var b strings.Builder

	b.WriteString(latexPreamble)
	b.WriteString("\n\\begin{document}\n\n")

	writeResumeHeader(&b, contact)
	writeResumeSkills(&b, content)
	writeResumeExperience(&b, content.Experience)
	writeResumeProjects(&b, content.Projects)
	writeResumeEducation(&b, education)

	b.WriteString("\n\\end{document}\n")

	return b.String()
}

func writeResumeHeader(b *strings.Builder, contact ResumeContact) {
	b.WriteString("\\begin{center}\n")
	fmt.Fprintf(
		b,
		"{\\Huge \\scshape %s} \\\\ \\vspace{3pt}\n\\small\n",
		escapeLaTeX(contact.Name),
	)

	var parts []string

	if contact.Phone != "" {
		parts = append(parts, fmt.Sprintf(
			"\\raisebox{-0.1\\height}\\faPhone\\ %s",
			escapeLaTeX(contact.Phone),
		))
	}

	if contact.Email != "" {
		parts = append(parts, fmt.Sprintf(
			"\\href{mailto:%s}{\\raisebox{-0.2\\height}\\faEnvelope\\ %s}",
			contact.Email,
			escapeLaTeX(contact.Email),
		))
	}

	if contact.LinkedIn != "" {
		parts = append(parts, fmt.Sprintf(
			"\\href{%s}{\\raisebox{-0.2\\height}\\faLinkedin\\ %s}",
			contact.LinkedIn,
			escapeLaTeX(stripScheme(contact.LinkedIn)),
		))
	}

	if contact.GitHub != "" {
		parts = append(parts, fmt.Sprintf(
			"\\href{%s}{\\raisebox{-0.2\\height}\\faGithub\\ %s}",
			contact.GitHub,
			escapeLaTeX(stripScheme(contact.GitHub)),
		))
	}

	b.WriteString(strings.Join(parts, " ~\n"))
	b.WriteString("\n\\vspace{-8pt}\n\\end{center}\n\n")
}

func writeResumeSkills(b *strings.Builder, content ResumeContent) {
	if len(content.Skills) == 0 && content.Achievement == "" {
		return
	}

	b.WriteString("\\section{Technical Skills / Achievements}\n")
	b.WriteString("\\begin{itemize}[leftmargin=0.15in, label={}]\n")
	b.WriteString("  \\small{\\item{\n")

	lines := make([]string, 0, len(content.Skills)+1)

	for _, s := range content.Skills {
		if strings.TrimSpace(s.Value) == "" {
			continue
		}

		lines = append(lines, fmt.Sprintf(
			"    \\textbf{%s:} %s",
			escapeLaTeX(s.Label),
			escapeLaTeX(s.Value),
		))
	}

	if content.Achievement != "" {
		lines = append(lines, "    "+escapeLaTeX(content.Achievement))
	}

	b.WriteString(strings.Join(lines, " \\\\\n"))
	b.WriteString("\n  }}\n\\end{itemize}\n\n")
}

func writeResumeExperience(b *strings.Builder, entries []ResumeExperience) {
	if len(entries) == 0 {
		return
	}

	b.WriteString("\\vspace{-20pt}\n\\section{Experience}\n")
	b.WriteString("\\resumeSubHeadingListStart\n")

	for i, e := range entries {
		heading := fmt.Sprintf("\\textbf{%s}", escapeLaTeX(e.Company))

		if e.Role != "" {
			heading += fmt.Sprintf(" $|$ \\emph{%s}", escapeLaTeX(e.Role))
		}

		if e.Tech != "" {
			heading += fmt.Sprintf(" \\textbf{(%s)}", escapeLaTeX(e.Tech))
		}

		fmt.Fprintf(
			b,
			"  \\resumeProjectHeading\n    {%s}{\\textbf{%s}}\n",
			heading,
			escapeLaTeX(e.Dates),
		)

		writeBullets(b, e.Bullets)

		if i < len(entries)-1 {
			b.WriteString("  \\vspace{-15pt}\n")
		}
	}

	b.WriteString("\\resumeSubHeadingListEnd\n\n")
}

func writeResumeProjects(b *strings.Builder, projects []ResumeProject) {
	if len(projects) == 0 {
		return
	}

	b.WriteString("\\vspace{-10pt}\n\\section{Projects}\n")
	b.WriteString("\\resumeSubHeadingListStart\n")

	for i, p := range projects {
		heading := fmt.Sprintf("\\textbf{%s}", escapeLaTeX(p.Name))

		if p.Tech != "" {
			heading += fmt.Sprintf(" $|$ \\emph{(%s)}", escapeLaTeX(p.Tech))
		}

		if p.GitHub != "" {
			heading += fmt.Sprintf(
				" $|$ \\href{%s}{GitHub}",
				p.GitHub,
			)
		}

		fmt.Fprintf(b, "  \\resumeProjectHeading\n    {%s}{}\n", heading)

		writeBullets(b, p.Bullets)

		if i < len(projects)-1 {
			b.WriteString("  \\vspace{-15pt}\n")
		}
	}

	b.WriteString("\\resumeSubHeadingListEnd\n\n")
}

func writeResumeEducation(b *strings.Builder, education []ResumeEducation) {
	if len(education) == 0 {
		return
	}

	b.WriteString("\\vspace{-10pt}\n\\section{Education}\n")
	b.WriteString("\\resumeSubHeadingListStart\n")

	for _, e := range education {
		fmt.Fprintf(
			b,
			"  \\resumeEducationEntry\n    {%s}{%s}\n    {%s}{}\n",
			escapeLaTeX(e.Institution),
			escapeLaTeX(e.Dates),
			escapeLaTeX(e.DegreeAndGPA),
		)
	}

	b.WriteString("\\resumeSubHeadingListEnd\n\n")
}

func writeBullets(b *strings.Builder, bullets []string) {
	nonEmpty := make([]string, 0, len(bullets))

	for _, bullet := range bullets {
		if strings.TrimSpace(bullet) != "" {
			nonEmpty = append(nonEmpty, bullet)
		}
	}

	if len(nonEmpty) == 0 {
		return
	}

	b.WriteString("  \\resumeItemListStart\n")

	for _, bullet := range nonEmpty {
		fmt.Fprintf(b, "    \\resumeItem{%s}\n", escapeLaTeX(bullet))
	}

	b.WriteString("  \\resumeItemListEnd\n")
}

// RenderResumePlainText renders the same tailored content as a plain-text
// resume. The application pipeline's readiness check needs a text artifact on
// disk, so this lets a single model call feed both the PDF and that artifact
// rather than paying for the text generation separately.
func RenderResumePlainText(
	contact ResumeContact,
	education []ResumeEducation,
	content ResumeContent,
) string {
	var b strings.Builder

	b.WriteString(contact.Name)
	b.WriteString("\n")

	var line []string
	for _, v := range []string{contact.Phone, contact.Email, contact.LinkedIn} {
		if strings.TrimSpace(v) != "" {
			line = append(line, v)
		}
	}
	if len(line) > 0 {
		b.WriteString(strings.Join(line, " | "))
		b.WriteString("\n")
	}

	if len(content.Skills) > 0 {
		b.WriteString("\nTECHNICAL SKILLS\n")
		for _, s := range content.Skills {
			fmt.Fprintf(&b, "%s: %s\n", s.Label, s.Value)
		}
	}

	if content.Achievement != "" {
		b.WriteString(content.Achievement)
		b.WriteString("\n")
	}

	if len(content.Experience) > 0 {
		b.WriteString("\nEXPERIENCE\n")
		for _, e := range content.Experience {
			fmt.Fprintf(&b, "%s — %s (%s) %s\n", e.Company, e.Role, e.Tech, e.Dates)
			for _, bullet := range e.Bullets {
				fmt.Fprintf(&b, "- %s\n", bullet)
			}
		}
	}

	if len(content.Projects) > 0 {
		b.WriteString("\nPROJECTS\n")
		for _, p := range content.Projects {
			fmt.Fprintf(&b, "%s (%s)\n", p.Name, p.Tech)
			for _, bullet := range p.Bullets {
				fmt.Fprintf(&b, "- %s\n", bullet)
			}
		}
	}

	if len(education) > 0 {
		b.WriteString("\nEDUCATION\n")
		for _, e := range education {
			fmt.Fprintf(&b, "%s, %s — %s\n", e.Institution, e.Dates, e.DegreeAndGPA)
		}
	}

	return b.String()
}

// stripScheme trims the protocol from a URL for display, so a header reads
// "linkedin.com/in/x" rather than "https://linkedin.com/in/x".
func stripScheme(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	return url
}
