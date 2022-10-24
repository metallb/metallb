module.exports = {
    rules: {
        // Ensure we have a blank line between the header and the body.
       'body-leading-blank': [2, 'always'],
       // Warn on lines longer than 72 chars. We usually don't want them for
       // text, but links are fine if they exceed. So, just warn.
       'body-max-line-length': [1, 'always', 72],
       // Ensure message body is not empty
       'body-empty': [2, 'never'],
       // Avoid having the 'Signed-off-by' message pass
       // for the body text by ensuring that the body
       // ends with a period.
       'body-full-stop': [2, 'always', '.'],
       // Ensure the header line doesn't end with a period.
       'header-full-stop': [2, 'never'],
    },
};
