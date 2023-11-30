document.addEventListener("DOMContentLoaded", async () => {
    const matches = window.matchMedia("(prefers-color-scheme: dark)").matches;
    updateFaviconStyle(matches);

    const newState = JSON.parse(document.getElementById("state").textContent);

    const params = new URLSearchParams(window.location.search);
    if (params.has("token")) {
        setToken(newState.key, params.get("token"));
    }

    window.history.replaceState(newState, "");
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (event) => {
    updateFaviconStyle(event.matches);
});

window.addEventListener("popstate", (event) => {
    updateCode(event.state);
    updatePage(event.state);
});


/* File Events */

document.getElementById("files").addEventListener("change", (e) => {
    const state = getState();
    const file = state.files[e.target.value];

    state.current_file = parseInt(e.target.value);
    window.history.replaceState(state, "");

    document.getElementById("code-edit").value = file.content;
    document.getElementById("code-view").innerHTML = file.formatted;
    document.getElementById("code-edit-count").innerText = `${file.content.length}`;
    document.getElementById("language").value = file.language;
})

document.getElementById("files").addEventListener("dblclick", (e) => {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = true;
    e.target.focus();
});

document.getElementById("files").addEventListener("focusout", (e) => {
    if (e.target.tagName.toLowerCase() !== "label") {
        return;
    }
    e.target.contentEditable = false;
});

document.getElementById("files").addEventListener("keypress", (e) => {
    if (e.key === "Enter") {
        e.preventDefault();
        if (e.target.contentEditable) {
            e.target.blur();
        }
    }
});

document.getElementById("files").addEventListener("input", (e) => {
    const state = getState();
    state.files[state.current_file].name = e.target.innerText;
    window.history.replaceState(state, "");
})

document.getElementById("file-add").addEventListener("click", (e) => {
    const state = getState();
    const index = state.files.length;

    const input = document.createElement("template");
    const label = document.createElement("template");
    input.innerHTML = `<input id="file-${index}" type="radio" name="files" value="${index}"/>`;
    label.innerHTML = `<label for="file-${index}">untitled${index}</label>`;

    e.target.parentElement.insertBefore(input.content.firstElementChild, e.target);
    const labelElement = e.target.parentElement.insertBefore(label.content.firstElementChild, e.target);

    state.files[index] = {
        name: `untitled${index}`,
        content: "",
        formatted: "",
        language: "auto"
    }
    window.history.replaceState(state, "")
    labelElement.click();
});


/* Code Edit Events */

document.getElementById("code-edit").addEventListener("keydown", (e) => {
    if (e.key !== "Tab" || e.shiftKey) {
        return;
    }
    e.preventDefault();

    const start = e.target.selectionStart;
    const end = e.target.selectionEnd;
    e.target.value = e.target.value.substring(0, start) + "\t" + e.target.value.substring(end);
    e.target.selectionStart = e.target.selectionEnd = start + 1;
});

document.getElementById("code-edit").addEventListener("input", (e) => {
    const state = getState();
    state.files[state.current_file].content = e.target.value;

    const count = e.target.value.length;
    const countElement = document.querySelector("#code-edit-count");
    countElement.innerHTML = count
    const maxElement = document.querySelector("#code-edit-max");
    if (!maxElement) return;
    if (count > maxElement.innerHTML) {
        countElement.classList.add("error");
    } else {
        countElement.classList.remove("error");
    }
});

document.getElementById("code-edit").addEventListener("paste", (event) => {
    const state = getState();
    state.files[state.current_file].content = event.target.value;
    updatePage(state);
    window.history.replaceState(state, "");
})

document.getElementById("code-edit").addEventListener("cut", (event) => {
    const state = getState();
    state.files[state.current_file].content = event.target.value;
    updatePage(state);
    window.history.replaceState(state, "");
})

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const state = getState();
    state.files[state.current_file].content = event.target.value;
    updatePage(state);
    window.history.replaceState(state, "");
})

/* Footer Events */

document.getElementById("version").addEventListener("change", (e) => {

});

document.getElementById("style").addEventListener("change", (e) => {
    const style = e.target.value;
    const theme = e.target.options.item(e.target.selectedIndex).dataset.theme;
    setCookie("style", style);
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.replace(theme === "dark" ? "light" : "dark", theme);
    const themeCssElement = document.getElementById("theme-css");

    const href = new URL(themeCssElement.href);
    href.searchParams.set("style", style);
    themeCssElement.href = href.toString();
});

document.getElementById("language").addEventListener("change", (e) => {
    window.gobin.files[window.gobin.current_file].language = e.target.value;
});

/* Keyboard Shortcut Events */

document.addEventListener("keydown", (event) => {
    const shortcuts = {s: "save", n: "new", e: "edit", d: "duplicate"};
    if (!event.ctrlKey || !(event.key in shortcuts)) return;
    doKeyboardAction(event, shortcuts[event.key]);
})

const doKeyboardAction = (event, elementName) => {
    event.preventDefault();
    if (document.querySelector(`#${elementName}`).disabled) return;
    document.querySelector(`#${elementName}`).click();
}

/* Navigation Action Button Events */

document.querySelector("#edit").addEventListener("click", async () => {
    if (document.querySelector("#edit").disabled) return;

    const {key, content, language} = getState();
    const {
        newState,
        url
    } = createState(hasPermission(getToken(key), "write") ? key : "", "", "edit", content, language);
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", url);
})

document.getElementById("save").addEventListener("click", async () => {
    if (document.querySelector("#save").disabled) {
        return;
    }
    const state = getState();
    if (state.mode !== "edit") {
        return;
    }
    const token = getToken(state.key);
    const saveButton = document.getElementById("save");
    saveButton.classList.add("loading");

    let response;
    if (state.key && token) {
        response = await fetch(`/documents/${key}?formatter=html${language ? `&language=${language || "auto"}` : ""}`, {
            method: "PATCH",
            body: content,
            headers: {
                Authorization: `Bearer ${token}`,
            }
        });
    } else {
        response = await fetch(`/documents?formatter=html${language ? `&language=${language || "auto"}` : ""}`, {
            method: "POST",
            body: content,
        });
    }
    saveButton.classList.remove("loading");

    let body = await response.text();
    try {
        body = JSON.parse(body);
    } catch (e) {
        body = {message: body};
    }
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error saving document:", response);
        return;
    }

    const {newState, url} = createState(body.key, "", "view", content, body.language);
    if (body.token) {
        setToken(body.key, body.token);
    }
    document.querySelector("#code-view").innerHTML = body.formatted;
    document.querySelector("#code-style").innerHTML = body.css;
    document.querySelector("#code-edit").value = body.data;
    document.querySelector("#language").value = body.language;

    const optionElement = document.createElement("option")
    optionElement.title = `${body.version_time}`;
    optionElement.value = body.version;
    optionElement.innerText = `${body.version_label}`;

    updateVersionSelect(-1);
    const versionElement = document.querySelector("#version")
    versionElement.insertBefore(optionElement, versionElement.firstChild);
    versionElement.value = body.version;

    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", url);
});

document.querySelector("#delete").addEventListener("click", async () => {
    if (document.querySelector("#delete").disabled) return;

    const {key} = getState();
    const token = getToken(key);
    if (!token) return;

    const deleteConfirm = window.confirm("Are you sure you want to delete this document? This action cannot be undone.")
    if (!deleteConfirm) return;

    const deleteButton = document.querySelector("#delete");
    deleteButton.classList.add("loading");
    let response = await fetch(`/documents/${key}`, {
        method: "DELETE",
        headers: {
            Authorization: `Bearer ${token}`
        }
    });
    deleteButton.classList.remove("loading");

    if (!response.ok) {
        let body = await response.text();
        try {
            body = JSON.parse(body);
        } catch (e) {
            body = {message: body};
        }
        showErrorPopup(body.message || response.statusText)
        console.error("error deleting document:", response);
        return;
    }
    deleteToken();
    const {newState, url} = createState("", "", "edit", "", "");
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", url);
})

document.querySelector("#copy").addEventListener("click", async () => {
    if (document.querySelector("#copy").disabled) return;

    const {content} = getState();
    if (!content) return;
    await navigator.clipboard.writeText(content);
})

document.querySelector("#raw").addEventListener("click", () => {
    if (document.querySelector("#raw").disabled) return;

    const {key, version} = getState();
    if (!key) return;
    window.open(`/raw/${key}${version ? `/versions/${version}` : ""}`, "_blank").focus();
})

document.querySelector("#share").addEventListener("click", async () => {
    if (document.querySelector("#share").disabled) return;

    const {key} = getState();
    const token = getToken(key);
    if (!hasPermission(token, "share")) {
        await navigator.clipboard.writeText(window.location.href);
        return;
    }

    document.querySelector("#share-permissions-write").checked = false;
    document.querySelector("#share-permissions-delete").checked = false;
    document.querySelector("#share-permissions-share").checked = false;

    document.querySelector("#share-dialog").showModal();
});

document.querySelector("#share-dialog-close").addEventListener("click", () => {
    document.querySelector("#share-dialog").close();
});

document.querySelector("#share-copy").addEventListener("click", async () => {
    const permissions = [];
    if (document.querySelector("#share-permissions-write").checked) {
        permissions.push("write");
    }
    if (document.querySelector("#share-permissions-delete").checked) {
        permissions.push("delete");
    }
    if (document.querySelector("#share-permissions-share").checked) {
        permissions.push("share");
    }

    if (permissions.length === 0) {
        await navigator.clipboard.writeText(window.location.href);
        document.querySelector("#share-dialog").close();
        return;
    }

    const {key} = getState();
    const token = getToken(key);

    const response = await fetch(`/documents/${key}/share`, {
        method: "POST",
        body: JSON.stringify({permissions: permissions}),
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`
        }
    });

    if (!response.ok) {
        const body = await response.json();
        showErrorPopup(body.message || response.statusText)
        console.error("error sharing document:", response);
        return;
    }

    const body = await response.json()
    const shareUrl = window.location.href + "?token=" + body.token;
    await navigator.clipboard.writeText(shareUrl);
    document.querySelector("#share-dialog").close();
});


document.querySelector("#language").addEventListener("change", async (event) => {
    const {key, version, mode, content} = getState();
    const {newState, url} = createState(key, version, mode, content, event.target.value);
    window.history.replaceState(newState, "", url);
    if (!key) return;
    await fetchDocument(key, version, event.target.value);
});

document.querySelector("#style").addEventListener("change", async (event) => {
    const {key, version, mode, language} = getState();
    const style = event.target.value;
    const theme = event.target.options.item(event.target.selectedIndex).dataset.theme;
    setCookie("style", style);
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.replace(theme === "dark" ? "light" : "dark", theme);
    if (!key || mode === "edit") {
        await fetchCSS(style);
        return;
    }
    await fetchDocument(key, version, language);
});

document.querySelector("#version").addEventListener("change", async (event) => {
    const {key, version} = getState();
    let newVersion = event.target.value;
    if (event.target.options.item(0).value === newVersion) {
        newVersion = "";
    }
    if (newVersion === version) return;

    const {newState, url} = await fetchDocument(key, newVersion);

    updateVersionSelect(event.target.selectedIndex);

    updateCode(newState);
    window.history.pushState(newState, "", url);
})

function updateVersionSelect(currentIndex) {
    const versionElement = document.querySelector("#version")
    for (let i = 0; i < versionElement.options.length; i++) {
        const element = versionElement.options.item(i);
        if (element.innerText.endsWith(" (current)")) {
            element.innerText = element.innerText.substring(0, element.innerText.length - 10);
        }
    }
    if (currentIndex !== versionElement.options.length - 1 && currentIndex !== -1) {
        versionElement.options.item(currentIndex).innerText += " (current)";
    }
}

async function fetchCSS(style) {
    const response = await fetch(`/assets/theme.css?style=${style}`, {
        method: "GET"
    });

    let body = await response.text();
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error fetching css:", response);
        return;
    }

    document.querySelector("#theme-style").innerHTML = body;
}

async function fetchDocument(key, version, language) {
    const response = await fetch(`/documents/${key}${version ? `/versions/${version}` : ""}?formatter=html${language ? `&language=${language}` : ""}`, {
        method: "GET"
    });

    let body = await response.text();
    try {
        body = JSON.parse(body);
    } catch (e) {
        body = {message: body};
    }
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error fetching document version:", response);
        return;
    }

    document.querySelector("#code-view").innerHTML = body.formatted;
    document.querySelector("#code-style").innerHTML = body.css;
    document.querySelector("#theme-style").innerHTML = body.theme_css;
    document.querySelector("#code-edit").value = body.data;
    document.querySelector("#language").value = body.language;

    return createState(key, `${body.version === 0 ? "" : body.version}`, "view", body.data, body.language);
}

function showErrorPopup(message) {
    const popup = document.getElementById("error-popup");
    popup.style.display = "block";
    popup.innerText = message || "Something went wrong.";
    setTimeout(() => popup.style.display = "none", 5000);
}

function getState() {
    return window.history.state;
}

function createState(key, version, mode, files) {
    return {
        newState: {key, version, mode, files},
        url: `/${key}${version ? `/${version}` : ""}${window.location.hash}`
    };
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

function updateCode(state) {
    if (!state) return;
    const {mode} = state;

    const codeElement = document.querySelector("#code");
    const codeEditElement = document.querySelector("#code-edit");

    if (mode === "view") {
        codeEditElement.style.display = "none";
        codeElement.style.display = "block";
        return;
    }
    codeEditElement.style.display = "block";
    codeElement.style.display = "none";
}

function updatePage(state) {
    if (!state) return;
    const {key, mode, files} = state;
    const token = getToken(key);
    // update page title
    if (key) {
        document.title = `gobin - ${key}`;
    } else {
        document.title = "gobin";
    }

    const saveButton = document.querySelector("#save");
    const editButton = document.querySelector("#edit");
    const deleteButton = document.querySelector("#delete");
    const copyButton = document.querySelector("#copy");
    const rawButton = document.querySelector("#raw");
    const shareButton = document.querySelector("#share");
    const versionSelect = document.querySelector("#version");
    versionSelect.disabled = versionSelect.options.length <= 1;
    if (mode === "view") {
        saveButton.disabled = true;
        editButton.disabled = false;
        deleteButton.disabled = !hasPermission(token, "delete");
        copyButton.disabled = false;
        rawButton.disabled = false;
        shareButton.disabled = false;
        return
    }
    saveButton.disabled = files.findIndex(file => file.content.length > 0) === -1;
    editButton.disabled = true;
    deleteButton.disabled = true;
    copyButton.disabled = true;
    rawButton.disabled = true;
    shareButton.disabled = true;
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
