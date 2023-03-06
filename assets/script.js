document.addEventListener("DOMContentLoaded", async () => {
    const path = window.location.pathname === "/" ? [] : window.location.pathname.slice(1).split("/")
    const key = path.length > 0 ? path[0] : ""
    const version = path.length > 1 ? path[1] : ""
    const params = new URLSearchParams(window.location.search);
    if (params.has("token")) {
        setToken(key, params.get("token"));
    }

    document.querySelector("#nav-btn").checked = false;

    let content = "", language = "";
    if (key) {
        content = document.querySelector("#code-edit").value;
        language = document.querySelector("#language").value;
    }
    const {newState, url} = createState(key, version, key ? "view" : "edit", content, language);
    updateCode(newState);
    updatePage(newState);
    window.history.replaceState(newState, "", url);
});

window.addEventListener("popstate", (event) => {
    updateCode(event.state);
    updatePage(event.state);
});

document.querySelector("#code-edit").addEventListener("keydown", (event) => {
    if (event.key !== "Tab" || event.shiftKey) {
        return;
    }
    event.preventDefault();

    const start = event.target.selectionStart;
    const end = event.target.selectionEnd;
    event.target.value = event.target.value.substring(0, start) + "\t" + event.target.value.substring(end);
    event.target.selectionStart = event.target.selectionEnd = start + 1;
});

document.querySelector("#code-edit").addEventListener("paste", (event) => {
    const codeEditElement = document.querySelector("#code-edit");
    const {key, version, language} = getState();
    const {newState, url} = createState(key, version, "edit", codeEditElement.value, language);
    updatePage(newState);
    window.history.replaceState(newState, "", url);
})

document.addEventListener("keydown", (event) => {
    if (!event.ctrlKey || !["s", "n", "e", "d"].includes(event.key)) return;
    doKeyboardAction(event, event.key);
})

const doKeyboardAction = (event, elementName) => {
    event.preventDefault();
    if (document.querySelector(`#${elementName}`).disabled) return;
    document.querySelector(`#${elementName}`).click();
}

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const {key, version, language} = getState();
    const {newState, url} = createState(key, version, "edit", event.target.value, language);
    updatePage(newState);
    window.history.replaceState(newState, "", url);
})

document.querySelector("#edit").addEventListener("click", async () => {
    if (document.querySelector("#edit").disabled) return;

    const {key, content, language} = getState();
    const {newState, url} = createState(hasPermission(getToken(key), "write") ? key : "", "", "edit", content, language);
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", url);
})

document.querySelector("#save").addEventListener("click", async () => {
    if (document.querySelector("#save").disabled) return;
    const {key, mode, content, language} = getState()
    if (mode !== "edit") return;
    const token = getToken(key);
    const saveButton = document.querySelector("#save");
    saveButton.classList.add("loading");

    let response;
    if (key && token) {
        response = await fetch(`/documents/${key}?render=html${language ? `&language=${language || "auto"}` : ""}`, {
            method: "PATCH",
            body: content,
            headers: {
                Authorization: `Bearer ${token}`,
            }
        });
    } else {
        response = await fetch(`/documents?render=html${language ? `&language=${language || "auto"}` : ""}`, {
            method: "POST",
            body: content,
        });
    }
    saveButton.classList.remove("loading");

    const body = await response.json();
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
        const body = await response.json();
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
    const {key, version, language} = getState();
    setCookie("style", event.target.value);
    await fetchDocument(key, version, language);
});

document.querySelector("#version").addEventListener("change", async (event) => {
    const {key, version} = getState();
    let newVersion = event.target.value;
    console.log(event.target.options.item(0).value, newVersion);
    if (event.target.options.item(0).value === newVersion) {
        newVersion = "";
    }
    if (newVersion === version) return;

    const {newState, url} = await fetchDocument(key, newVersion);
    updateCode(newState);
    window.history.pushState(newState, "", url);
})

async function fetchDocument(key, version, language) {
    const response = await fetch(`/documents/${key}${version ? `/versions/${version}` : ""}?render=html${language ? `&language=${language}` : ""}`, {
        method: "GET"
    });

    const body = await response.json();
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error fetching document version:", response);
        return;
    }

    document.querySelector("#code-view").innerHTML = body.formatted;
    document.querySelector("#code-style").innerHTML = body.css;
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

function createState(key, version, mode, content, language) {
    return {newState: {key, version, mode, content: content.trim(), language}, url: `/${key}${version ? `/${version}` : ""}${window.location.hash}`};
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

function deleteToken() {
    const {key} = getState();
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
    const {key, mode, content} = state;
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
        saveButton.style.display = "none";
        editButton.disabled = false;
        editButton.style.display = "block";
        deleteButton.disabled = !hasPermission(token, "delete");
        copyButton.disabled = false;
        rawButton.disabled = false;
        shareButton.disabled = false;
        return
    }
    saveButton.disabled = content === "";
    saveButton.style.display = "block";
    editButton.disabled = true;
    editButton.style.display = "none";
    deleteButton.disabled = true;
    copyButton.disabled = true;
    rawButton.disabled = true;
    shareButton.disabled = true;
}
