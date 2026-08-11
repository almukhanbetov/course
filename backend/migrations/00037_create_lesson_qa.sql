-- +goose Up

-- lesson_questions is a student's question about a specific lesson —
-- Stage 20A. course_id is denormalized from lesson_id's chain (lesson ->
-- module -> course) purely so eligibility/ownership/moderation queries
-- never need a three-way join to find "which course does this question
-- belong to" — resolved once at creation time via internal/ownership,
-- mirroring how other domains denormalize a derived id rather than forcing
-- every read through a join.
CREATE TABLE lesson_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id UUID NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    published BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Backs the primary "questions for this lesson" read (lesson_id), the
-- instructor/admin moderation queue scoped to a course (course_id), and the
-- IDOR-safe own-question delete (user_id).
CREATE INDEX idx_lesson_questions_lesson_id ON lesson_questions (lesson_id);
CREATE INDEX idx_lesson_questions_course_id ON lesson_questions (course_id);
CREATE INDEX idx_lesson_questions_user_id ON lesson_questions (user_id);

-- question_answers is a reply to a lesson_questions row — from the asker's
-- peers, the course's own instructor, or an admin. is_instructor_answer
-- marks an authoritative reply (course-owning instructor, or admin) so the
-- frontend can visually distinguish it from a peer-student answer.
CREATE TABLE question_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES lesson_questions (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    is_instructor_answer BOOLEAN NOT NULL DEFAULT false,
    published BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- idx_question_answers_question_id backs the second of the two queries the
-- no-N+1 list join relies on: "all answers for this page of questions" via
-- WHERE question_id = ANY($1).
CREATE INDEX idx_question_answers_question_id ON question_answers (question_id);
CREATE INDEX idx_question_answers_user_id ON question_answers (user_id);

-- +goose Down
DROP TABLE question_answers;
DROP TABLE lesson_questions;
