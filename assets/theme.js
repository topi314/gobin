window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (event) => {
    updateFaviconStyle(event.matches);
});

document.addEventListener("DOMContentLoaded", () => {
    const matches = window.matchMedia("(prefers-color-scheme: dark)").matches;
    updateFaviconStyle(matches);
    setStyle(localStorage.getItem("stylePreference") || (matches ? "atom-one-dark.min.css" : "atom-one-light.min.css"));
});

function updateFaviconStyle(matches) {
    const faviconElement = document.querySelector(`link[rel="icon"]`)
    if (matches) {
        faviconElement.href = "/assets/favicon.png";
        return
    }
    faviconElement.href = "/assets/favicon-light.png";
}

function setStyle(style) {
    localStorage.setItem("stylePreference", style)
    const highlightJSElement = document.querySelector(`link[title="Highlight.js Style"]`);
    if (highlightJSElement) {
        highlightJSElement.href = `/assets/styles/${style}`;
    }
    const styleElement = document.querySelector("#style");
    if (styleElement) {
        styleElement.value = style;
    }

    const theme = style.includes("dark") ? "dark" : "light";
    const rootClassList = document.querySelector(":root").classList;
    rootClassList.add(theme);
    rootClassList.remove(theme === "dark" ? "light" : "dark");
}