document.addEventListener("DOMContentLoaded", async () => {
    updateFaviconStyle(window.matchMedia("(prefers-color-scheme: dark)").matches);
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (event) => {
    updateFaviconStyle(event.matches);
});

async function onFileAdd() {
    await htmx.ajax("POST", "/documents", {
        event: "add-file",
        swap: "multi:#"
    })
}

function onFileDblClick(e) {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = true;
    e.target.focus();
}

function onFileFocusOut(e) {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = false;
}

function onFileKeyPress(e) {
    if (e.which === 13) {
        e.preventDefault();
        if (e.target.contentEditable) {
            e.target.blur();
        }
    }
}

function onFileSelect(e) {
    for (const tab of document.getElementsByClassName("file-tab")) {
        tab.style.display = "none";
    }

    const tab = document.getElementById(e.target.value);
    if (tab) {
        tab.style.display = "flex";
    }
}

function onCodeEditKeyDown(event) {
    if (event.key !== "Tab" || event.shiftKey) {
        return;
    }
    event.preventDefault();

    const start = event.target.selectionStart;
    const end = event.target.selectionEnd;
    event.target.value = event.target.value.substring(0, start) + "\t" + event.target.value.substring(end);
    event.target.selectionStart = event.target.selectionEnd = start + 1;
}

function onCodeEditInput(event) {
    const count = event.target.value.length;

    const countElement = document.querySelector("#code-edit-count");
    countElement.innerHTML = count
    const maxElement = document.querySelector("#code-edit-max");
    if (!maxElement) return;
    if (count > maxElement.innerHTML) {
        countElement.classList.add("error");
    } else {
        countElement.classList.remove("error");
    }
}

function onStyleChange(event) {
    const style = event.target.value;
    const theme = event.target.options.item(event.target.selectedIndex).dataset.theme;
    setCookie("style", style);
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.replace(theme === "dark" ? "light" : "dark", theme);
    const themeCssElement = document.getElementById("theme-css");

    const href = new URL(themeCssElement.href);
    href.searchParams.set("style", style);
    themeCssElement.href = href.toString();
}

function onVersionChange(event) {

}


function onLanguageChange(event) {

}


function getToken(key) {
    const documents = localStorage.getItem("documents")
    if (!documents) return ""
    const token = JSON.parse(documents)[key]
    if (!token) return ""

    return token
}

function setToken(key, token) {
    let documents = localStorage.getItem("documents")
    if (!documents) {
        documents = "{}"
    }
    const parsedDocuments = JSON.parse(documents)
    parsedDocuments[key] = token
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
}

function deleteToken(key) {
    const documents = localStorage.getItem("documents");
    if (!documents) return;
    const parsedDocuments = JSON.parse(documents);
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments));
}

function hasPermission(token, permission) {
    if (!token) return false;
    const tokenSplit = token.split(".")
    if (tokenSplit.length !== 3) return false;
    return JSON.parse(atob(tokenSplit[1])).permissions.includes(permission);
}

function updateFaviconStyle(matches) {
    const faviconElement = document.querySelector(`link[rel="icon"]`)
    if (matches) {
        faviconElement.href = "/assets/favicon.png";
        return
    }
    faviconElement.href = "/assets/favicon-light.png";
}

function getCookie(name) {
    let matches = document.cookie.match(new RegExp(
        "(?:^|; )" + name.replace(/([.$?*|{}()\[\]\\\/+^])/g, '\\$1') + "=([^;]*)"
    ));
    return matches ? decodeURIComponent(matches[1]) : undefined;
}

function setCookie(name, value, options = {}) {
    options = {
        path: "/",
        sameSite: "strict",
        ...options
    };

    if (options.expires instanceof Date) {
        options.expires = options.expires.toUTCString();
    }

    let updatedCookie = encodeURIComponent(name) + "=" + encodeURIComponent(value);

    for (let optionKey in options) {
        updatedCookie += "; " + optionKey;
        let optionValue = options[optionKey];
        if (optionValue !== true) {
            updatedCookie += "=" + optionValue;
        }
    }

    document.cookie = updatedCookie;
}
