hljs.listLanguages().forEach((language) => {
    const option = document.createElement("option");
    option.value = language;
    option.innerText = language;

    const languageElement = document.querySelector("#language")
    if (languageElement.value === language) {
        languageElement.removeChild(languageElement.querySelector(`option[value="${language}"]`));
        option.selected = true;
    }

    languageElement.appendChild(option);
});

document.addEventListener("DOMContentLoaded", () => {
    const key = window.location.pathname === "/" ? "" : window.location.pathname.slice(1);
    const params = new URLSearchParams(window.location.search);
    if (params.has("token")) {
        setUpdateToken(key, params.get("token"));
    }

    let newState;
    let url;
    if (key) {
        const content = document.querySelector("#code-view").innerText
        const language = document.querySelector("#language").value;
        newState = {key: key, mode: "view", content: content, language: language};
        url = `/${key}`;
    } else {
        newState = {key: "", mode: "edit", content: "", language: ""};
        url = "/";
    }

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
})

document.addEventListener("keydown", (event) => {
    if (!event.ctrlKey) return;
    if (event.key !== "s" && event.key !== "n" && event.key !== "e" && event.key !== "d") return;
    doKeyboardAction(event, event.key);
})

const doKeyboardAction = (event, elementName) => {
    event.preventDefault();
    if (document.querySelector(`#${elementName}`).disabled) return;
    document.querySelector(`#${elementName}`).click();
}

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const {key, language} = getState();
    const newState = {key: key, mode: "edit", content: event.target.value, language: language};
    window.history.replaceState(newState, "", `/${key}`);
    updatePage(newState);
})

document.querySelector("#edit").addEventListener("click", async () => {
    if (document.querySelector("#edit").disabled) return;

    const {key, content, language} = getState();
    let newState;
    let url;
    if (getUpdateToken(key) === "") {
        newState = {key: "", mode: "edit", content: content, language: language};
        url = "/";
    } else {
        newState = {key: key, mode: "edit", content: content, language: language};
        url = `/${key}`;
    }

    updateCode(newState);
    updatePage(newState);

    window.history.pushState(newState, "", url);
})

document.querySelector("#save").addEventListener("click", async () => {
    if (document.querySelector("#save").disabled) return;
    const {key, mode, content, language} = getState()
    if (mode !== "edit") return;
    const updateToken = getUpdateToken(key);
    const saveButton = document.querySelector("#save");
    saveButton.classList.add("loading");

    let response;
    if (key && updateToken) {
        response = await fetch(`/documents/${key}`, {
            method: "PATCH",
            body: content,
            headers: {
                Authorization: updateToken,
                Language: language
            }
        });
    } else {
        response = await fetch("/documents", {
            method: "POST",
            body: content,
            headers: {
                Language: language
            }
        });
    }
    saveButton.classList.remove("loading");

    const body = await response.json();
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error saving document:", response);
        return;
    }

    const newState = { key: body.key, mode: "view", content: body.data, language: body.language };
    setUpdateToken(body.key, body.update_token);
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", `/${body.key}`);
});

document.querySelector("#delete").addEventListener("click", async () => {
    if (document.querySelector("#delete").disabled) return;

    const {key} = getState();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        return;
    }

    const deleteConfirm = window.confirm("Are you sure you want to delete this document? This action cannot be undone.")
    if (!deleteConfirm) return;

    const deleteButton = document.querySelector("#delete");
    deleteButton.classList.add("loading");
    let response = await fetch(`/documents/${key}`, {
        method: "DELETE", headers: {
            Authorization: updateToken
        }
    });
    deleteButton.classList.remove("loading");

    if (!response.ok) {
        const body = await response.json();
        showErrorPopup(body.message || response.statusText)
        console.error("error deleting document:", response);
        return;
    }
    deleteUpdateToken();
    const newState = {key: "", mode: "edit", content: "", language: ""};
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", "/");
})

document.querySelector("#copy").addEventListener("click", async () => {
    if (document.querySelector("#copy").disabled) return;

    const {content} = getState();
    if (!content) return;
    await navigator.clipboard.writeText(content);
})

document.querySelector("#raw").addEventListener("click", () => {
    if (document.querySelector("#raw").disabled) return;

    const {key} = getState();
    if (!key) return;
    window.open(`/raw/${key}`, "_blank").focus();
})

document.querySelector("#share").addEventListener("click", async () => {
    if (document.querySelector("#share").disabled) return;

    const {key} = getState();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        await navigator.clipboard.writeText(window.location.href);
        return;
    }

    document.querySelector("#share-permissions").checked = false;
    document.querySelector("#share-url").value = window.location.href;
    document.querySelector("#share-dialog").showModal();
});

document.querySelector("#share-dialog-close").addEventListener("click", () => {
    document.querySelector("#share-dialog").close();
});

document.querySelector("#share-permissions").addEventListener("change", (event) => {
    const {key} = getState();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        return;
    }

    const shareUrl = document.querySelector("#share-url");
    if (event.target.checked) {
        shareUrl.value = `${window.location.href}?token=${updateToken}`;
        return;
    }
    shareUrl.value = window.location.href;
});

document.querySelector("#share-url").addEventListener("click", () => {
    document.querySelector("#share-url").select();
});

document.querySelector("#share-copy").addEventListener("click", async () => {
    const shareUrl = document.querySelector("#share-url");
    await navigator.clipboard.writeText(shareUrl.value);
    document.querySelector("#share-dialog").close();
});


document.querySelector("#language").addEventListener("change", (event) => {
    const {key, mode, content} = getState();
    const newState = {key: key, mode: mode, content: content, language: event.target.value};
    highlightCode(newState);
    window.history.replaceState(newState, "", window.location.pathname);
});

document.querySelector("#style").addEventListener("change", (event) => {
    setStyle(event.target.value);
});

function showErrorPopup(message) {
    const popup = document.getElementById("error-popup");
    popup.style.display = "block";
    popup.innerText = message || "Something went wrong.";
    setTimeout(() => popup.style.display = "none", 5000);
}


function getState() {
    return window.history.state;
}

function getUpdateToken(key) {
    const documents = localStorage.getItem("documents")
    if (!documents) return ""
    const updateToken = JSON.parse(documents)[key]
    if (!updateToken) return ""

    return updateToken
}

function setUpdateToken(key, updateToken) {
    let documents = localStorage.getItem("documents")
    if (!documents) {
        documents = "{}"
    }
    const parsedDocuments = JSON.parse(documents)
    parsedDocuments[key] = updateToken
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
}

function deleteUpdateToken() {
    const {key} = getState();
    const documents = localStorage.getItem("documents");
    if (!documents) return;
    const parsedDocuments = JSON.parse(documents);
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments));
}

function updateCode(state) {
    const {mode, content} = state;

    const codeElement = document.querySelector("#code");
    const codeEditElement = document.querySelector("#code-edit");
    const codeViewElement = document.querySelector("#code-view");

    if (mode === "view") {
        codeEditElement.style.display = "none";
        codeEditElement.value = "";
        codeViewElement.innerText = content;
        codeElement.style.display = "block";
        highlightCode(state);
        return;
    }
    codeEditElement.value = content;
    codeEditElement.style.display = "block";
    codeViewElement.innerText = "";
    codeElement.style.display = "none";
}

function updatePage(state) {
    const {key, mode, content} = state;
    const updateToken = getUpdateToken(key);
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
    if (mode === "view") {
        saveButton.disabled = true;
        saveButton.style.display = "none";
        editButton.disabled = false;
        editButton.style.display = "block";
        if (updateToken) {
            deleteButton.disabled = false;
        }
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

function highlightCode(state) {
    const {content, language} = state;
    let result;
    if (language && language !== "auto") {
        result = hljs.highlight(content, {
            language: language, ignoreIllegals: true
        });
    } else {
        result = hljs.highlightAuto(content);
    }
    if (result.language === undefined) {
        result.language = "plaintext";
    }

    if (result.language !== language) {
        state.language = result.language;
    }

    const codeViewElement = document.querySelector("#code-view");
    codeViewElement.innerHTML = result.value;
    codeViewElement.className = "hljs language-" + result.language;

    document.querySelector("#language").value = result.language;

    if (result.value) {
        hljs.initLineNumbersOnLoad({singleLine: true});
    }
}